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
	"strconv"

	"github.com/DataDog/orchestrion/internal/toolexec/aspect/linkdeps"
	"github.com/DataDog/orchestrion/internal/toolexec/importcfg"
	"github.com/DataDog/orchestrion/internal/toolexec/proxy"
	"github.com/rs/zerolog"
)

// OnCompileMain only performs changes when compiling the "main" package, adding blank imports for
// any linkdeps dependencies that are not yet satisfied by the importcfg file (this is the case for
// link-time dependencies implied by use of the go:linkname directive, which are used to avoid
// creating circular import dependencies).
// This ensures that the relevant packages' `init` (if any) are appropriately run, and that the
// linker automatically picks up these dependencies when creating the full binary.
func (Weaver) OnCompileMain(ctx context.Context, cmd *proxy.CompileCommand) error {
	if cmd.Flags.Package != "main" {
		return nil
	}

	log := zerolog.Ctx(ctx).With().Str("phase", "compile(main)").Logger()
	ctx = log.WithContext(ctx)

	reg, err := importcfg.ParseFile(cmd.Flags.ImportCfg)
	if err != nil {
		return fmt.Errorf("parsing %q: %w", cmd.Flags.ImportCfg, err)
	}

	linkDeps, err := linkdeps.FromImportConfig(&reg)
	if err != nil {
		return fmt.Errorf("reading %s closure from %s: %w", linkdeps.Filename, cmd.Flags.ImportCfg, err)
	}

	if linkDeps.Empty() {
		// Nothing was added, we're done!
		return nil
	}

	newDeps := linkDeps.Dependencies()

	// Add package resolutions of link-time dependencies to the importcfg file:
	stack := append(make([]string, 0, len(newDeps)), newDeps...)
	for len(stack) > 0 {
		// Pop from the stack of things to process...
		linkDepPath := stack[len(stack)-1]
		stack = stack[:len(stack)-1]

		deps, err := resolvePackageFiles(ctx, linkDepPath, cmd.WorkDir)
		if err != nil {
			return fmt.Errorf("resolving %q: %w", linkDepPath, err)
		}

		for p, a := range deps {
			if _, found := reg.PackageFile[p]; found {
				continue
			}
			log.Debug().Str("import-path", p).Str("archive", a).Msg("Recording resolved " + linkdeps.Filename + " dependency")
			reg.PackageFile[p] = a

			// The package may have its own link-time dependencies we need to resolve
			tDeps, err := linkdeps.FromArchive(a)
			if err != nil {
				return fmt.Errorf("reading %s from %s[%s]: %w", linkdeps.Filename, p, a, err)
			}
			for _, tDep := range tDeps.Dependencies() {
				if reg.PackageFile[tDep] != "" || slices.Contains(stack, tDep) {
					// Already resolved, or already going to be resolved...
					continue
				}
				stack = append(stack, tDep)     // Push it to the stack
				newDeps = append(newDeps, tDep) // Record it as asynthetic import to add
				cmd.LinkDeps.Add(tDep)          // Record it as a link-time dependency
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
	genDecl := &ast.GenDecl{Tok: token.IMPORT, Specs: make([]ast.Spec, len(newDeps))}
	fileAST := &ast.File{Name: ast.NewIdent("main"), Decls: []ast.Decl{genDecl}, Imports: make([]*ast.ImportSpec, len(newDeps))}
	slices.Sort(newDeps) // Consistent order for deterministic output
	for idx, path := range newDeps {
		spec := &ast.ImportSpec{Name: ast.NewIdent("_"), Path: &ast.BasicLit{Kind: token.STRING, Value: strconv.Quote(path)}}
		genDecl.Specs[idx] = spec
		fileAST.Imports[idx] = spec
	}

	genDir := filepath.Join(filepath.Dir(cmd.Flags.Output), "orchestrion", "src", "synthetic")
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
