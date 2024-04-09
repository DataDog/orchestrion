// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package aspect

import (
	"fmt"
	"os"
	"os/exec"
	"path"
	"regexp"

	"github.com/datadog/orchestrion/internal/injector"
	"github.com/datadog/orchestrion/internal/injector/builtin"
	"github.com/datadog/orchestrion/internal/injector/typed"
	"github.com/datadog/orchestrion/internal/toolexec/processors"
	"github.com/datadog/orchestrion/internal/toolexec/processors/aspect/linkdeps"
	"github.com/datadog/orchestrion/internal/toolexec/proxy"
)

var (
	weavingDenyList = []*regexp.Regexp{
		regexp.MustCompile("^github.com/datadog/orchestrion(?:/.+)?$"),
		regexp.MustCompile("^gopkg.in/DataDog/dd-trace-go.v1(?:/.+)?$"),
	}
)

func (w Weaver) OnCompile(cmd *proxy.CompileCommand) error {
	for _, deny := range weavingDenyList {
		if deny.MatchString(w.ImportPath) {
			// No weaving in those packages!
			return nil
		}
	}

	orchestrionDir := path.Join(path.Dir(cmd.Flags.Output), "orchestrion")
	injector, err := injector.New(cmd.SourceDir, injector.Options{
		Aspects: builtin.Aspects[:],
		ModifiedFile: func(file string) string {
			return path.Join(orchestrionDir, "src", path.Base(file))
		},
		PreserveLineInfo: true,
	})
	if err != nil {
		return fmt.Errorf("creating injector for %s (in %q): %w", w.ImportPath, cmd.SourceDir, err)
	}

	references := typed.ReferenceMap{}
	for _, gofile := range cmd.GoFiles() {
		res, err := injector.InjectFile(gofile, map[string]string{"httpmode": "wrap"})
		if err != nil {
			return fmt.Errorf("weaving aspects in %q: %w", gofile, err)
		}

		if !res.Modified {
			continue
		}

		if err := cmd.ReplaceParam(gofile, res.Filename); err != nil {
			return fmt.Errorf("replacing %q with %q: %w", gofile, res.Filename, err)
		}

		references.Merge(res.References)
	}

	if len(references) == 0 {
		return nil
	}

	reg, err := processors.ParseImportConfig(cmd.Flags.ImportCfg)
	if err != nil {
		return fmt.Errorf("parsing %q: %w", cmd.Flags.ImportCfg, err)
	}

	var (
		linkDeps   linkdeps.LinkDeps
		regUpdated bool
	)
	for depImportPath, kind := range references {
		if _, ok := reg.PackageFile[depImportPath]; ok {
			// Already part of natural dependencies, nothing to do...
			continue
		}

		linkDeps.Add(depImportPath)

		if kind == typed.ImportStatement {
			// Imported packages need to be provided in the compilation's importcfg file
			deps, err := resolvePackageFiles(depImportPath)
			if err != nil {
				return fmt.Errorf("resolving woven dependency on %s: %w", depImportPath, err)
			}
			for dep, archive := range deps {
				if _, ok := reg.PackageFile[dep]; ok {
					// Already part of natural dependencies, nothing to do...
					continue
				}
				reg.PackageFile[dep] = archive
				regUpdated = true
			}
		}
	}

	if linkDeps.Empty() {
		// There are no synthetic dependencies, so we don't need to write an updated importcfg or add
		// extra objects in the output file.
		return nil
	}

	if regUpdated {
		// Creating updated version of the importcfg file, with new dependencies
		if err := writeUpdatedImportConfig(reg, cmd.Flags.ImportCfg); err != nil {
			return fmt.Errorf("writing updated %q: %w", cmd.Flags.ImportCfg, err)
		}
	}

	// Write the link.deps file and add it to the output object once the compilation has completed.
	linkDepsFile := path.Join(orchestrionDir, linkdeps.LinkDepsFilename)
	if err := linkDeps.WriteFile(linkDepsFile); err != nil {
		return fmt.Errorf("writing %s file: %w", linkdeps.LinkDepsFilename, err)
	}
	cmd.OnClose(func() error {
		child := exec.Command("go", "tool", "pack", "r", cmd.Flags.Output, linkDepsFile)
		if err := child.Run(); err != nil {
			return fmt.Errorf("running %q: %w", child.Args, err)
		}
		return nil
	})

	return nil
}

func writeUpdatedImportConfig(reg *processors.PackageRegister, filename string) (err error) {
	const dotOriginal = ".original"

	if err := os.Rename(filename, filename+dotOriginal); err != nil {
		return fmt.Errorf("renaming to %q: %w", path.Base(filename)+dotOriginal, err)
	}

	file, err := os.Create(filename)
	if err != nil {
		return fmt.Errorf("opening for writing: %w", err)
	}
	defer func() {
		if closeErr := file.Close(); closeErr != nil && err == nil {
			err = fmt.Errorf("closing: %w", closeErr)
		}
	}()

	if _, err := reg.WriteTo(file); err != nil {
		return fmt.Errorf("writing: %w", err)
	}

	return nil
}
