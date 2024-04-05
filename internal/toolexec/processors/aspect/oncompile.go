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
	"github.com/datadog/orchestrion/internal/toolexec/proxy"
)

const (
	nameLinkDeps = "link.deps"
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
		regUpdated bool
		linkDeps   *os.File
	)
	for depImportPath, kind := range references {
		switch kind {
		case typed.ImportStatement:
			if _, ok := reg.PackageFile[depImportPath]; ok {
				// Already part of natural dependencies, nothing to do...
				continue
			}

			deps, err := resolvePackageFiles(depImportPath)
			if err != nil {
				return fmt.Errorf("resolving woven dependency on %s: %w", depImportPath, err)
			}

			regUpdated = true
			for dep, archive := range deps {
				if _, ok := reg.PackageFile[dep]; ok {
					// Already part of natural dependencies, nothing to do...
					continue
				}
				reg.PackageFile[dep] = archive
				regUpdated = true
			}

			fallthrough // For writing into link.deps file
		case typed.RelocationTarget:
			if linkDeps == nil {
				// Lazily create the file (no file --> no link-only dependencies)
				linkDepsPath := path.Join(orchestrionDir, nameLinkDeps)
				linkDeps, err = os.Create(linkDepsPath)
				if err != nil {
					return fmt.Errorf("creating %s file: %w", nameLinkDeps, err)
				}
				defer linkDeps.Close()
				cmd.OnClose(func() error {
					return exec.Command("go", "tool", "pack", "r", cmd.Flags.Output, linkDepsPath).Run()
				})
			}
			if _, err := fmt.Fprintln(linkDeps, depImportPath); err != nil {
				return fmt.Errorf("writing into %s file: %w", nameLinkDeps, err)
			}
		}
	}

	if regUpdated {
		if err := os.Rename(cmd.Flags.ImportCfg, cmd.Flags.ImportCfg+".original"); err != nil {
			return fmt.Errorf("renaming %q: %w", cmd.Flags.ImportCfg, err)
		}

		file, err := os.Create(cmd.Flags.ImportCfg)
		if err != nil {
			return fmt.Errorf("opening %q for writing: %w", cmd.Flags.ImportCfg, err)
		}
		defer file.Close()
		if _, err := reg.WriteTo(file); err != nil {
			return fmt.Errorf("writing updated %q: %w", cmd.Flags.ImportCfg, err)
		}
	}

	return nil
}
