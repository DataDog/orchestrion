// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package aspect

import (
	"errors"
	"fmt"
	"go/ast"
	"go/format"
	"go/token"
	"os"
	"path/filepath"
	"slices"
	"strconv"

	"github.com/DataDog/orchestrion/internal/log"
	"github.com/DataDog/orchestrion/internal/toolexec/aspect/linkdeps"
	"github.com/DataDog/orchestrion/internal/toolexec/importcfg"
	"github.com/DataDog/orchestrion/internal/toolexec/proxy"
)

// OnCompileMain only performs changes when compiling the "main" package, adding blank imports for
// any linkdeps dependencies that are not yet satisfied by the importcfg file (this is the case for
// link-time dependencies implied by use of the go:linkname directive, which are used to avoid
// creating circular import dependencies).
// This ensures that the relevant packages' `init` (if any) are appropriately run, and that the
// linker automatically picks up these dependencies when creating the full binary.
func (Weaver) OnCompileMain(cmd *proxy.CompileCommand) error {
	if cmd.Flags.Package != "main" {
		return nil
	}

	log.SetContext("PHASE", "compile(main)")
	defer log.SetContext("PHASE", "")

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
	for _, linkDepPath := range newDeps {
		deps, err := resolvePackageFiles(linkDepPath, cmd.WorkDir)
		if err != nil {
			return fmt.Errorf("resolving %q: %w", linkDepPath, err)
		}
		for p, a := range deps {
			if _, found := reg.PackageFile[p]; found {
				continue
			}
			log.Debugf("Recording resolved %s dependency: %q => %q\n", linkdeps.Filename, p, a)
			reg.PackageFile[p] = a
		}
	}

	// We back up the original ImportCfg file only if there's not already such a file (could have been created by OnCompile)
	backupFile := cmd.Flags.ImportCfg + ".original"
	if _, err := os.Stat(backupFile); errors.Is(err, os.ErrNotExist) {
		log.Tracef("Backing up original %q\n", cmd.Flags.ImportCfg)
		if err := os.Rename(cmd.Flags.ImportCfg, backupFile); err != nil {
			return fmt.Errorf("renaming %q: %w", cmd.Flags.ImportCfg, err)
		}
	}
	log.Tracef("Writing updated %q\n", cmd.Flags.ImportCfg)
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
	log.Tracef("Writing new blank imports source file %q\n", genFile)
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

	log.Debugf("Adding synthetic source file to command: %q\n", genFile)
	cmd.AddFiles([]string{genFile})

	return nil
}
