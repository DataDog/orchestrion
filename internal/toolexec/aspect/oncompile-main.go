// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package aspect

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/datadog/orchestrion/internal/log"
	"github.com/datadog/orchestrion/internal/toolexec/aspect/linkdeps"
	"github.com/datadog/orchestrion/internal/toolexec/importcfg"
	"github.com/datadog/orchestrion/internal/toolexec/proxy"
	"github.com/dave/jennifer/jen"
)

// OnCompileMain only performs changes when compiling the "main" package, adding blank imports for
// any linkdeps dependencies that are not yet satisfied by the importcfg file (this is the case for
// link-time dependencies implied by use of the go:linkname directive, which are used to avoid
// creating circular import dependencies).
// This ensures that the relevant packages' `init` (if any) are appropriately run, and that the
// linker automatically picks up these dependencies when creating the full binary.
func (w Weaver) OnCompileMain(cmd *proxy.CompileCommand) error {
	if cmd.Flags.Package != "main" {
		return nil
	}

	log.SetContext("PHASE", "compile(main)")
	defer log.SetContext("PHASE", "")

	reg, err := importcfg.ParseFile(cmd.Flags.ImportCfg)
	if err != nil {
		return fmt.Errorf("parsing %q: %w", cmd.Flags.ImportCfg, err)
	}

	var addImports []string
	for importPath, archive := range reg.PackageFile {
		linkDeps, err := linkdeps.FromArchive(archive)
		if err != nil {
			return fmt.Errorf("reading %s from %q: %w", linkdeps.LinkDepsFilename, importPath, err)
		}

		log.Debugf("Processing %s dependencies from %s[%s]...\n", linkdeps.LinkDepsFilename, importPath, archive)
		for _, depPath := range linkDeps.Dependencies() {
			if arch, found := reg.PackageFile[depPath]; found {
				log.Debugf("Already satisfied %s dependency: %q => %q\n", linkdeps.LinkDepsFilename, depPath, arch)
				continue
			}

			deps, err := resolvePackageFiles(depPath, cmd.WorkDir)
			if err != nil {
				return fmt.Errorf("resolving %q: %w", depPath, err)
			}
			for p, a := range deps {
				if _, found := reg.PackageFile[p]; !found {
					log.Debugf("Recording resolved %s dependency: %q => %q\n", linkdeps.LinkDepsFilename, p, a)
					reg.PackageFile[p] = a
				}
			}
			addImports = append(addImports, depPath)
		}
	}

	if len(addImports) == 0 {
		// Nothing was added, we're done!
		return nil
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

	source := jen.NewFile("main")
	for _, imp := range addImports {
		source.Anon(imp)
	}

	genDir := filepath.Join(filepath.Dir(cmd.Flags.Output), "orchestrion", "src", "synthetic")
	genFile := filepath.Join(genDir, "link_deps_imports.go")
	log.Tracef("Writing new blank imports source file %q\n", genFile)
	if err := os.MkdirAll(genDir, 0o755); err != nil {
		return fmt.Errorf("creating directory %q: %w", genDir, err)
	}
	if err := source.Save(genFile); err != nil {
		return fmt.Errorf("writing generated code to %q: %w", genFile, err)
	}

	log.Debugf("Adding synthetic source file to command: %q\n", genFile)
	cmd.AddFiles([]string{genFile})

	return nil
}
