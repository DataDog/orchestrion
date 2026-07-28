// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package aspect

import (
	"context"
	"errors"
	"fmt"
	"go/ast"
	"go/format"
	"go/token"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strconv"
	"strings"

	"github.com/DataDog/dd-trace-go/v2/ddtrace/tracer"
	"github.com/DataDog/orchestrion/internal/jobserver/pkgs"
	"github.com/DataDog/orchestrion/internal/toolexec/aspect/linkdeps"
	"github.com/DataDog/orchestrion/internal/toolexec/importcfg"
	"github.com/DataDog/orchestrion/internal/toolexec/proxy"
	"github.com/rs/zerolog"
)

// SyntheticPackageName is the name of the synthetic package that will be created when the compilation of the main
// package is performed. This folder contains blank imports for all link-time dependencies that are not already
// in the build tree
var SyntheticPackageName = "synthetic"

type pendingLinkDep struct {
	path   string
	parent string
	kind   linkdeps.DependencyKind
}

// OnCompileMain only performs changes when compiling the "main" package, adding blank imports for
// any linkdeps dependencies that are not yet satisfied by the importcfg file (this is the case for
// link-time dependencies implied by use of the go:linkname directive, which are used to avoid
// creating circular import dependencies).
// This ensures that the relevant packages' `init` (if any) are appropriately run, and that the
// linker automatically picks up these dependencies when creating the full binary.
func (w Weaver) OnCompileMain(ctx context.Context, cmd *proxy.CompileCommand) (err error) {
	if cmd.Flags.Package != "main" {
		return nil
	}
	isTestMain := cmd.TestMain() && strings.HasSuffix(w.ImportPath, ".test")
	if isTestMain && pkgs.ResolvingTestVariants() {
		// The nested test main only exists to make cmd/go build the affected library variants.
		// Processing its synthetic dependencies would recursively issue the same resolution request.
		return nil
	}

	span, ctx := tracer.StartSpanFromContext(ctx, "Weaver.OnCompileMain",
		tracer.ResourceName(w.ImportPath),
	)
	defer func() { span.Finish(tracer.WithError(err)) }()

	log := zerolog.Ctx(ctx).With().Str("phase", "compile(main)").Logger()
	ctx = log.WithContext(ctx)

	reg, err := importcfg.ParseFile(ctx, cmd.Flags.ImportCfg)
	if err != nil {
		return fmt.Errorf("parsing %q: %w", cmd.Flags.ImportCfg, err)
	}

	testVariantFor := ""
	if isTestMain {
		testVariantFor = strings.TrimSuffix(w.ImportPath, ".test")
		cmd.MarkTestMain(testVariantFor)
	}

	stack, err := initialLinkDependencies(ctx, &reg)
	if err != nil {
		return fmt.Errorf("reading %s closure from %s: %w", linkdeps.Filename, cmd.Flags.ImportCfg, err)
	}
	if len(stack) == 0 {
		return nil
	}

	newDeps := make([]string, 0, len(stack))
	pending := make(map[string]pendingLinkDep, len(stack))
	processed := make(map[string]pkgs.ResolvedArchive)
	paths := make([]string, 0, len(stack))
	for _, dep := range stack {
		newDeps = append(newDeps, dep.path)
		pending[dep.path] = dep
		paths = append(paths, dep.path)
	}

	// Add package resolutions of link-time dependencies to the importcfg file:
	for len(paths) > 0 {
		path := paths[len(paths)-1]
		paths = paths[:len(paths)-1]
		item := pending[path]
		delete(pending, path)

		deps, err := resolvePackageFilesForTest(ctx, item.path, testVariantFor, cmd.WorkDir)
		if err != nil {
			return fmt.Errorf("resolving %q: %w", item.path, err)
		}
		for path, archive := range deps {
			if current, found := processed[path]; !found || current.ForTest == "" || archive.ForTest != "" {
				processed[path] = archive
			}
		}
		if err := rejectResolvedSyntheticVariantDependency(item.parent, item.path, testVariantFor, item.kind == linkdeps.ImportDependency, deps); err != nil {
			return err
		}
		changed, err := mergeResolvedArchives(&reg, deps, testVariantFor)
		if err != nil {
			return err
		}
		for p, archive := range changed {
			log.Debug().Str("import-path", p).Str("archive", archive).Msg("Recording resolved " + linkdeps.Filename + " dependency")
		}

		for p, resolved := range deps {
			archive := resolved.ExportFile
			// The package may have its own link-time dependencies we need to resolve.
			tDeps, err := linkdeps.FromArchive(ctx, archive)
			if err != nil {
				return fmt.Errorf("reading %s from %s[%s]: %w", linkdeps.Filename, p, archive, err)
			}
			for _, tDep := range tDeps.Dependencies() {
				kind := tDeps.Kind(tDep)
				candidate := pendingLinkDep{path: tDep, parent: p, kind: kind}
				if current, found := pending[tDep]; found {
					cmd.LinkDeps.Add(tDep, candidate.kind)
					pending[tDep] = strongestPendingLinkDep(current, candidate)
					continue
				}
				if previous, found := processed[tDep]; found {
					if err := rejectSyntheticVariantDependency(candidate.parent, candidate.path, testVariantFor, candidate.kind == linkdeps.ImportDependency, previous); err != nil {
						return err
					}
					continue
				}
				if reg.PackageFile[tDep] != "" {
					continue
				}
				cmd.LinkDeps.Add(tDep, candidate.kind)
				pending[tDep] = candidate
				paths = append(paths, tDep)
				newDeps = append(newDeps, tDep)
			}
		}
	}

	// We back up the original ImportCfg file only if there's not already such a file (could have been created by OnCompile)
	backupFile := cmd.Flags.ImportCfg + ".original"
	if _, err := os.Stat(backupFile); errors.Is(err, os.ErrNotExist) {
		log.Trace().Str("path", cmd.Flags.ImportCfg).Msg("Backing up original file")
		if err := os.Rename(cmd.Flags.ImportCfg, backupFile); err != nil {
			return fmt.Errorf("renaming %q: %w", cmd.Flags.ImportCfg, err)
		}
	}
	log.Trace().Str("path", cmd.Flags.ImportCfg).Msg("Writing updated file")
	if err := reg.WriteFile(cmd.Flags.ImportCfg); err != nil {
		return fmt.Errorf("writing updated %q: %w", cmd.Flags.ImportCfg, err)
	}

	// Generate a synthetic source file with blank imports to link-time
	// dependencies, so the linker actually sees them.
	slices.Sort(newDeps) // Consistent order for deterministic output
	genDecl := &ast.GenDecl{Tok: token.IMPORT, Specs: make([]ast.Spec, len(newDeps))}
	fileAST := &ast.File{Name: ast.NewIdent("main"), Decls: []ast.Decl{genDecl}, Imports: make([]*ast.ImportSpec, len(newDeps))}
	for idx, path := range newDeps {
		spec := &ast.ImportSpec{Name: ast.NewIdent("_"), Path: &ast.BasicLit{Kind: token.STRING, Value: strconv.Quote(path)}}
		genDecl.Specs[idx] = spec
		fileAST.Imports[idx] = spec
	}

	genDir := filepath.Join(filepath.Dir(cmd.Flags.Output), "orchestrion", "src", SyntheticPackageName)
	genFile := filepath.Join(genDir, "link_deps_imports.go")
	log.Trace().Str("path", genFile).Msg("Writing new blank imports source file")
	if err := os.MkdirAll(genDir, 0o755); err != nil {
		return fmt.Errorf("creating directory %q: %w", genDir, err)
	}

	file, err := os.Create(genFile)
	if err != nil {
		return fmt.Errorf("create %q: %w", genFile, err)
	}
	defer file.Close()
	if err := format.Node(file, token.NewFileSet(), fileAST); err != nil {
		return fmt.Errorf("formatting generated code for %s: %w", genFile, err)
	}

	log.Debug().Str("path", genFile).Msg("Adding synthetic source file to command")
	cmd.AddFiles([]string{genFile})

	return nil
}

func initialLinkDependencies(ctx context.Context, reg *importcfg.ImportConfig) ([]pendingLinkDep, error) {
	byPath := make(map[string]pendingLinkDep)
	for parent, archive := range reg.PackageFile {
		deps, err := linkdeps.FromArchive(ctx, archive)
		if err != nil {
			return nil, fmt.Errorf("reading %s from %s=%s: %w", linkdeps.Filename, parent, archive, err)
		}
		for _, path := range deps.Dependencies() {
			kind := deps.Kind(path)
			if _, satisfied := reg.PackageFile[path]; satisfied {
				continue
			}
			candidate := pendingLinkDep{path: path, parent: parent, kind: kind}
			if current, found := byPath[path]; found {
				candidate = strongestPendingLinkDep(current, candidate)
			}
			byPath[path] = candidate
		}
	}

	result := make([]pendingLinkDep, 0, len(byPath))
	for _, dep := range byPath {
		result = append(result, dep)
	}
	sort.Slice(result, func(i int, j int) bool { return result[i].path < result[j].path })
	return result, nil
}

func strongestPendingLinkDep(left pendingLinkDep, right pendingLinkDep) pendingLinkDep {
	if right.kind > left.kind || (right.kind == left.kind && left.parent == "" && right.parent != "") {
		return right
	}
	return left
}
