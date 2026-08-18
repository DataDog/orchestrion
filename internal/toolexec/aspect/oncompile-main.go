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
	if pkgs.ResolvingTestVariants() && cmd.Flags.Package == "main" && w.isTestMain() {
		// The nested test main only exists to make cmd/go build affected variants.
		// Its source may come from the build cache without the _testmain.go filename.
		return nil
	}
	// Both signals are required: cmd.TestMain validates the generated source,
	// while w.isTestMain validates a variant-free ".test" identity; package
	// names may themselves end in ".test".
	isTestMain := cmd.TestMain() && w.isTestMain()

	spanOptions := []tracer.StartSpanOption{tracer.ResourceName(w.ImportPath)}
	if w.Variant != "" {
		spanOptions = append(spanOptions, tracer.Tag("variant", w.Variant))
	}
	span, ctx := tracer.StartSpanFromContext(ctx, "Weaver.OnCompileMain", spanOptions...)
	defer func() { span.Finish(tracer.WithError(err)) }()

	logContext := zerolog.Ctx(ctx).With().Str("phase", "compile(main)")
	if w.Variant != "" {
		logContext = logContext.Str("variant", w.Variant)
	}
	log := logContext.Logger()
	ctx = log.WithContext(ctx)

	reg, err := importcfg.ParseFile(ctx, cmd.Flags.ImportCfg)
	if err != nil {
		return fmt.Errorf("parsing %q: %w", cmd.Flags.ImportCfg, err)
	}

	testVariantFor := ""
	if isTestMain {
		testVariantFor = w.packageUnderTest()
		cmd.MarkTestMain(testVariantFor)
	}

	resolveTestTargetProvenance := newTestTargetProvenanceResolver(ctx, testVariantFor, cmd.WorkDir)
	reversePackageFiles, reverseRoots, err := resolveReverseTestVariants(ctx, &reg, testVariantFor, cmd.WorkDir, resolveTestTargetProvenance)
	if err != nil {
		return err
	}
	if len(reversePackageFiles) > 0 {
		cmd.SetTestMainPackageFiles(reversePackageFiles)
	}
	for _, root := range reverseRoots {
		cmd.AddTestMainReverseRoot(root)
	}
	stack, err := initialLinkDependencies(ctx, &reg, testVariantFor, resolveTestTargetProvenance, reversePackageFiles)
	if err != nil {
		return fmt.Errorf("reading %s closure from %s: %w", linkdeps.Filename, cmd.Flags.ImportCfg, err)
	}
	if len(stack) == 0 {
		if len(reversePackageFiles) > 0 {
			return writeMainImportConfig(log, &reg, cmd.Flags.ImportCfg)
		}
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
		requiresReverse, err := resolvedClosureImportsTarget(ctx, deps, testVariantFor)
		if err != nil {
			return fmt.Errorf("checking resolved closure for %q: %w", item.path, err)
		}
		if testVariantFor != "" && requiresReverse {
			variants, err := resolveReversePackageFilesForTest(ctx, item.path, testVariantFor, reg.PackageFile[testVariantFor], cmd.WorkDir)
			if err != nil {
				return fmt.Errorf("rebuilding synthetic importer %q for tests of %q: %w", item.path, testVariantFor, err)
			}
			deps = variants
			if reversePackageFiles == nil {
				reversePackageFiles = make(map[string]string)
				cmd.SetTestMainPackageFiles(reversePackageFiles)
			}
			for path, archive := range variants {
				reversePackageFiles[path] = archive.ExportFile
			}
			cmd.AddTestMainReverseRoot(item.path)
		}
		for path, archive := range deps {
			if current, found := processed[path]; !found || current.ForTest == "" || archive.ForTest != "" {
				processed[path] = archive
			}
		}
		parentRebuilt := reversePackageFiles[item.parent] != "" || processed[item.parent].ForTest == testVariantFor
		if !parentRebuilt {
			if err := rejectResolvedSyntheticVariantDependency(item.parent, item.path, testVariantFor, item.kind == linkdeps.ImportDependency, deps); err != nil {
				return err
			}
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
				if tDep == testVariantFor && kind == linkdeps.ImportDependency && resolved.ForTest == testVariantFor {
					continue
				}
				candidate := pendingLinkDep{path: tDep, parent: p, kind: kind}
				if current, found := pending[tDep]; found {
					cmd.LinkDeps.Add(tDep, candidate.kind)
					pending[tDep] = strongestPendingLinkDep(current, candidate)
					continue
				}
				if previous, found := processed[tDep]; found {
					if resolved.ForTest == testVariantFor {
						continue
					}
					if err := rejectSyntheticVariantDependency(candidate.parent, candidate.path, testVariantFor, candidate.kind == linkdeps.ImportDependency, previous); err != nil {
						return err
					}
					continue
				}
				if reg.PackageFile[tDep] != "" {
					if resolved.ForTest == testVariantFor {
						continue
					}
					if tDep == testVariantFor && kind == linkdeps.ImportDependency {
						selected, err := resolveTestTargetProvenance()
						if err != nil {
							return fmt.Errorf("resolving test target provenance for %q: %w", testVariantFor, err)
						}
						if err := rejectSatisfiedSyntheticDependency(p, tDep, testVariantFor, kind, selected); err != nil {
							return err
						}
					}
					continue
				}
				cmd.LinkDeps.Add(tDep, candidate.kind)
				pending[tDep] = candidate
				paths = append(paths, tDep)
				newDeps = append(newDeps, tDep)
			}
		}
	}

	if err := writeMainImportConfig(log, &reg, cmd.Flags.ImportCfg); err != nil {
		return err
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

func writeMainImportConfig(log zerolog.Logger, reg *importcfg.ImportConfig, filename string) error {
	// We back up the original ImportCfg file only if there is not already such a
	// file; OnCompile may have created it before main-specific processing.
	backupFile := filename + ".original"
	if _, err := os.Stat(backupFile); errors.Is(err, os.ErrNotExist) {
		log.Trace().Str("path", filename).Msg("Backing up original file")
		if err := os.Rename(filename, backupFile); err != nil {
			return fmt.Errorf("renaming %q: %w", filename, err)
		}
	} else if err != nil {
		return fmt.Errorf("checking import configuration backup %q: %w", backupFile, err)
	}
	log.Trace().Str("path", filename).Msg("Writing updated file")
	if err := reg.WriteFile(filename); err != nil {
		return fmt.Errorf("writing updated %q: %w", filename, err)
	}
	return nil
}

func resolvedClosureImportsTarget(ctx context.Context, archives pkgs.ResolveResponse, target string) (bool, error) {
	if target == "" {
		return false, nil
	}
	for path, resolved := range archives {
		if resolved.ForTest == target {
			continue
		}
		deps, err := linkdeps.FromArchive(ctx, resolved.ExportFile)
		if err != nil {
			return false, fmt.Errorf("reading %s from %s[%s]: %w", linkdeps.Filename, path, resolved.ExportFile, err)
		}
		if deps.Contains(target) && deps.Kind(target) == linkdeps.ImportDependency {
			return true, nil
		}
	}
	return false, nil
}

func resolveReverseTestVariants(
	ctx context.Context,
	reg *importcfg.ImportConfig,
	testVariantFor string,
	workDir string,
	resolveTestTargetProvenance func() (pkgs.ResolvedArchive, error),
) (map[string]string, []string, error) {
	if testVariantFor == "" {
		return nil, nil, nil
	}
	selected, err := resolveTestTargetProvenance()
	if err != nil {
		return nil, nil, fmt.Errorf("resolving test target provenance for %q: %w", testVariantFor, err)
	}
	if selected.ForTest != testVariantFor {
		return nil, nil, nil
	}
	authoritative := reg.PackageFile[testVariantFor]
	if authoritative == "" {
		return nil, nil, fmt.Errorf("test-main import configuration is missing the authoritative package-under-test archive %q", testVariantFor)
	}

	parents := make([]string, 0, len(reg.PackageFile))
	for parent := range reg.PackageFile {
		parents = append(parents, parent)
	}
	sort.Strings(parents)
	result := make(map[string]string)
	var roots []string
	for _, parent := range parents {
		if parent == testVariantFor {
			continue
		}
		archive := reg.PackageFile[parent]
		deps, err := linkdeps.FromArchive(ctx, archive)
		if err != nil {
			return nil, nil, fmt.Errorf("reading %s from %s=%s: %w", linkdeps.Filename, parent, archive, err)
		}
		if !deps.Contains(testVariantFor) || deps.Kind(testVariantFor) != linkdeps.ImportDependency {
			continue
		}
		variants, err := resolveReversePackageFilesForTest(ctx, parent, testVariantFor, authoritative, workDir)
		if err != nil {
			return nil, nil, fmt.Errorf("rebuilding synthetic importer %q for tests of %q: %w", parent, testVariantFor, err)
		}
		if variants[parent].ForTest != testVariantFor {
			return nil, nil, fmt.Errorf("reverse test variant resolution did not rebuild synthetic importer %q for %q", parent, testVariantFor)
		}
		changed, err := mergeResolvedArchives(reg, variants, testVariantFor)
		if err != nil {
			return nil, nil, err
		}
		roots = append(roots, parent)
		for path, archive := range changed {
			result[path] = archive
		}
	}
	if len(result) == 0 {
		return nil, nil, nil
	}
	return result, roots, nil
}

func initialLinkDependencies(
	ctx context.Context,
	reg *importcfg.ImportConfig,
	testVariantFor string,
	resolveTestTargetProvenance func() (pkgs.ResolvedArchive, error),
	reversePackageFiles map[string]string,
) ([]pendingLinkDep, error) {
	byPath := make(map[string]pendingLinkDep)
	parents := make([]string, 0, len(reg.PackageFile))
	for parent := range reg.PackageFile {
		parents = append(parents, parent)
	}
	sort.Strings(parents)
	for _, parent := range parents {
		archive := reg.PackageFile[parent]
		deps, err := linkdeps.FromArchive(ctx, archive)
		if err != nil {
			return nil, fmt.Errorf("reading %s from %s=%s: %w", linkdeps.Filename, parent, archive, err)
		}
		for _, path := range deps.Dependencies() {
			kind := deps.Kind(path)
			_, satisfied := reg.PackageFile[path]
			if path == testVariantFor && !satisfied {
				return nil, fmt.Errorf("test-main import configuration is missing the authoritative package-under-test archive %q", testVariantFor)
			}
			if satisfied {
				if path == testVariantFor && kind == linkdeps.ImportDependency && reversePackageFiles[parent] == archive {
					continue
				}
				if path == testVariantFor && kind == linkdeps.ImportDependency {
					selected, err := resolveTestTargetProvenance()
					if err != nil {
						return nil, fmt.Errorf("resolving test target provenance for %q: %w", testVariantFor, err)
					}
					if err := rejectSatisfiedSyntheticDependency(parent, path, testVariantFor, kind, selected); err != nil {
						return nil, err
					}
				}
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
