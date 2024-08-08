// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package aspect

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"

	"github.com/datadog/orchestrion/internal/injector"
	"github.com/datadog/orchestrion/internal/injector/aspect"
	"github.com/datadog/orchestrion/internal/injector/builtin"
	"github.com/datadog/orchestrion/internal/injector/typed"
	"github.com/datadog/orchestrion/internal/log"
	"github.com/datadog/orchestrion/internal/toolexec/aspect/linkdeps"
	"github.com/datadog/orchestrion/internal/toolexec/importcfg"
	"github.com/datadog/orchestrion/internal/toolexec/proxy"
)

type specialCaseBehavior int

const (
	neverWeave specialCaseBehavior = iota
	weaveTracerInternal
)

var (
	weavingSpecialCase = map[*regexp.Regexp]specialCaseBehavior{
		regexp.MustCompile(`^github\.com/datadog/orchestrion(?:/.+)?$`):  neverWeave,
		regexp.MustCompile(`^gopkg\.in/DataDog/dd-trace-go.v1(?:/.+)?$`): weaveTracerInternal,
		regexp.MustCompile(`^github\.com/DataDog/go-tuf/client$`):        neverWeave,
	}
)

func (w Weaver) OnCompile(cmd *proxy.CompileCommand) error {
	log.SetContext("PHASE", "compile")
	defer log.SetContext("PHASE", "")

	aspects := builtin.Aspects[:]
	for pattern, override := range weavingSpecialCase {
		if pattern.MatchString(w.ImportPath) {
			if override == neverWeave {
				log.Debugf("Not weaving aspects in %q to prevent circular instrumentation\n", w.ImportPath)
				// No weaving in those packages!
				return nil
			} else {
				log.Debugf("Enabling tracer-internal mode for %q\n", w.ImportPath)
				shortList := make([]aspect.Aspect, 0, len(aspects))
				for _, aspect := range aspects {
					if aspect.TracerInternal {
						shortList = append(shortList, aspect)
					}
				}
				aspects = shortList
			}
		}
	}

	imports, err := importcfg.ParseFile(cmd.Flags.ImportCfg)
	if err != nil {
		return err
	}

	orchestrionDir := filepath.Join(filepath.Dir(cmd.Flags.Output), "orchestrion")
	injector := injector.Injector{
		Aspects:          aspects,
		RootConfig:       map[string]string{"httpmode": "wrap"},
		PreserveLineInfo: true,
		LookupImport:     imports.Lookup,
		ImportPath:       w.ImportPath,
		GoVersion:        cmd.Flags.GoVersion,
		ModifiedFile: func(file string) string {
			return filepath.Join(orchestrionDir, "src", cmd.Flags.Package, filepath.Base(file))
		},
	}

	goFiles := cmd.GoFiles()
	results, err := injector.InjectFiles(goFiles)
	if err != nil {
		return err
	}

	references := typed.ReferenceMap{}
	for idx, gofile := range cmd.GoFiles() {
		res := results[idx]
		if !res.Modified {
			continue
		}

		log.Debugf("Modified source code: %q => %q\n", gofile, res.Filename)
		if err := cmd.ReplaceParam(gofile, res.Filename); err != nil {
			return fmt.Errorf("replacing %q with %q: %w", gofile, res.Filename, err)
		}

		references.Merge(res.References)
	}

	if references.Count() == 0 {
		return nil
	}

	reg, err := importcfg.ParseFile(cmd.Flags.ImportCfg)
	if err != nil {
		return fmt.Errorf("parsing %q: %w", cmd.Flags.ImportCfg, err)
	}

	var (
		linkDeps   linkdeps.LinkDeps
		regUpdated bool
	)
	for depImportPath, kind := range references.Map() {
		if depImportPath == "unsafe" {
			// Unsafe isn't like other go packages, and it does not have an associated archive file.
			continue
		}

		if archive, ok := reg.PackageFile[depImportPath]; ok {
			deps, err := linkdeps.FromArchive(archive)
			if err != nil {
				return fmt.Errorf("reading %s from %q: %w", linkdeps.LinkDepsFilename, depImportPath, err)
			}
			log.Debugf("Processing %s dependencies from %s[%s]...", linkdeps.LinkDepsFilename, depImportPath, archive)
			for _, tDep := range deps.Dependencies() {
				if _, found := reg.PackageFile[tDep]; !found {
					log.Debugf("Copying %s dependency on %q inherited from %q\n", linkdeps.LinkDepsFilename, tDep, depImportPath)
					linkDeps.Add(tDep)
				}
			}

			// Already part of natural dependencies, nothing to do...
			continue
		}

		log.Debugf("Recording synthetic dependency: %q => %v\n", depImportPath, kind)
		linkDeps.Add(depImportPath)

		if kind == typed.ImportStatement {
			// Imported packages need to be provided in the compilation's importcfg file
			deps, err := resolvePackageFiles(depImportPath, cmd.WorkDir)
			if err != nil {
				return fmt.Errorf("resolving woven dependency on %s: %w", depImportPath, err)
			}
			for dep, archive := range deps {
				deps, err := linkdeps.FromArchive(archive)
				if err != nil {
					return fmt.Errorf("reading %s from %s[%s]: %w", linkdeps.LinkDepsFilename, dep, archive, err)
				}
				log.Debugf("Processing %s dependencies from %s...\n", linkdeps.LinkDepsFilename, dep)
				for _, tDep := range deps.Dependencies() {
					if _, found := reg.PackageFile[tDep]; !found {
						log.Debugf("Copying transitive %s dependency on %q inherited from %q via %q\n", linkdeps.LinkDepsFilename, tDep, depImportPath, dep)
						linkDeps.Add(tDep)
					}
				}

				if _, ok := reg.PackageFile[dep]; ok {
					// Already part of natural dependencies, nothing to do...
					continue
				}
				log.Debugf("Recording transitive dependency of %q: %q => %q\n", depImportPath, dep, archive)
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
	linkDepsFile := filepath.Join(orchestrionDir, linkdeps.LinkDepsFilename)
	if err := linkDeps.WriteFile(linkDepsFile); err != nil {
		return fmt.Errorf("writing %s file: %w", linkdeps.LinkDepsFilename, err)
	}
	cmd.OnClose(func() error {
		log.Debugf("Adding %s file into %q\n", linkdeps.LinkDepsFilename, cmd.Flags.Output)
		child := exec.Command("go", "tool", "pack", "r", cmd.Flags.Output, linkDepsFile)
		if err := child.Run(); err != nil {
			return fmt.Errorf("running %q: %w", child.Args, err)
		}
		return nil
	})

	return nil
}

func writeUpdatedImportConfig(reg importcfg.ImportConfig, filename string) (err error) {
	const dotOriginal = ".original"

	log.Tracef("Backing up original %q\n", filename)
	if err := os.Rename(filename, filename+dotOriginal); err != nil {
		return fmt.Errorf("renaming to %q: %w", filepath.Base(filename)+dotOriginal, err)
	}

	log.Debugf("Writing updated %q\n", filename)
	if err := reg.WriteFile(filename); err != nil {
		return fmt.Errorf("writing: %w", err)
	}

	return nil
}
