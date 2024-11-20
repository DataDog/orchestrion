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
	"strings"

	"github.com/DataDog/orchestrion/internal/injector"
	"github.com/DataDog/orchestrion/internal/injector/aspect"
	"github.com/DataDog/orchestrion/internal/injector/builtin"
	"github.com/DataDog/orchestrion/internal/injector/typed"
	"github.com/DataDog/orchestrion/internal/log"
	"github.com/DataDog/orchestrion/internal/toolexec/aspect/linkdeps"
	"github.com/DataDog/orchestrion/internal/toolexec/importcfg"
	"github.com/DataDog/orchestrion/internal/toolexec/proxy"
)

type (
	specialCase struct {
		path     string
		prefix   bool
		behavior behaviorOverride
	}

	behaviorOverride int
)

const (
	// noOverride does not change the injector behavior, but prevents further
	// rules from being applied.
	noOverride behaviorOverride = iota
	// neverWeave completely disables injecting into the designated package
	// path(s).
	neverWeave
	// weaveTracerInternal limits weaving to only aspects that have the
	// `tracer-internal` flag set.
	weaveTracerInternal
)

// matches returns true if the importPath is matched by this special case.
func (sc *specialCase) matches(importPath string) bool {
	if importPath == sc.path {
		return true
	}
	return sc.prefix && strings.HasPrefix(importPath, sc.path+"/")
}

// weavingSpecialCase defines special behavior to be applied to certain package
// paths. They are evaluated in order, and the first matching override is
// applied, stopping evaluation of any further overrides.
var weavingSpecialCase = []specialCase{
	{path: "github.com/DataDog/orchestrion/runtime", prefix: true, behavior: noOverride},
	{path: "github.com/DataDog/orchestrion", prefix: true, behavior: neverWeave},
	{path: "gopkg.in/DataDog/dd-trace-go.v1", prefix: true, behavior: weaveTracerInternal},
	{path: "github.com/DataDog/go-tuf/client", prefix: false, behavior: neverWeave},
}

func (w Weaver) OnCompile(cmd *proxy.CompileCommand) (result error) {
	log.SetContext("PHASE", "compile")
	defer log.SetContext("PHASE", "")

	imports, err := importcfg.ParseFile(cmd.Flags.ImportCfg)
	if err != nil {
		return fmt.Errorf("parsing %q: %w", cmd.Flags.ImportCfg, err)
	}

	var linkDeps linkdeps.LinkDeps
	for _, archiveName := range imports.PackageFile {
		deps, err := linkdeps.FromArchive(archiveName)
		if err != nil {
			return fmt.Errorf("reading %s from %q: %w", linkdeps.LinkDepsFilename, archiveName, err)
		}
		for _, depPath := range deps.Dependencies() {
			if _, found := imports.PackageFile[depPath]; found {
				continue
			}
			linkDeps.Add(depPath)
		}
	}

	orchestrionDir := filepath.Join(filepath.Dir(cmd.Flags.Output), "orchestrion")

	defer func() {
		if result != nil {
			return
		}

		// Write the link.deps file and add it to the output object once the compilation has completed.
		if err := os.MkdirAll(orchestrionDir, 0o755); err != nil {
			result = fmt.Errorf("making directory %s: %w", orchestrionDir, err)
			return
		}
		linkDepsFile := filepath.Join(orchestrionDir, linkdeps.LinkDepsFilename)
		if err := linkDeps.WriteFile(linkDepsFile); err != nil {
			result = fmt.Errorf("writing %s file: %w", linkdeps.LinkDepsFilename, err)
			return
		}
		cmd.OnClose(func() error {
			log.Debugf("Adding %s file into %q\n", linkdeps.LinkDepsFilename, cmd.Flags.Output)
			child := exec.Command("go", "tool", "pack", "r", cmd.Flags.Output, linkDepsFile)
			if err := child.Run(); err != nil {
				return fmt.Errorf("running %q: %w", child.Args, err)
			}
			return nil
		})
	}()

	aspects := builtin.Aspects[:]
	for _, sc := range weavingSpecialCase {
		if !sc.matches(w.ImportPath) {
			continue
		}

		switch sc.behavior {
		case neverWeave:
			log.Debugf("Not weaving aspects in %q to prevent circular instrumentation\n", w.ImportPath)
			return nil

		case weaveTracerInternal:
			log.Debugf("Enabling tracer-internal mode for %q\n", w.ImportPath)
			shortList := make([]aspect.Aspect, 0, len(aspects))
			for _, aspect := range aspects {
				if aspect.TracerInternal {
					shortList = append(shortList, aspect)
				}
			}
			aspects = shortList

		case noOverride:
			// No-op

		default:
			// Unreachable
			panic(fmt.Sprintf("un-handled behavior override: %d", sc.behavior))
		}

		// We matched an override; so we'll not evaluate any other.
		break
	}

	injector := injector.Injector{
		Aspects:    aspects,
		RootConfig: map[string]string{"httpmode": "wrap"},
		Lookup:     imports.Lookup,
		ImportPath: w.ImportPath,
		GoVersion:  cmd.Flags.Lang,
		ModifiedFile: func(file string) string {
			return filepath.Join(orchestrionDir, "src", cmd.Flags.Package, filepath.Base(file))
		},
	}

	goFiles := cmd.GoFiles()
	results, goLang, err := injector.InjectFiles(goFiles)
	if err != nil {
		return err
	}

	if err := cmd.SetLang(goLang); err != nil {
		return err
	}

	references := typed.ReferenceMap{}
	for gofile, modFile := range results {
		log.Debugf("Modified source code: %q => %q\n", gofile, modFile.Filename)
		if err := cmd.ReplaceParam(gofile, modFile.Filename); err != nil {
			return fmt.Errorf("replacing %q with %q: %w", gofile, modFile.Filename, err)
		}

		references.Merge(modFile.References)
	}

	if references.Count() == 0 {
		return nil
	}

	var regUpdated bool
	for depImportPath, kind := range references.Map() {
		if depImportPath == "unsafe" {
			// Unsafe isn't like other go packages, and it does not have an associated archive file.
			continue
		}

		if archive, ok := imports.PackageFile[depImportPath]; ok {
			deps, err := linkdeps.FromArchive(archive)
			if err != nil {
				return fmt.Errorf("reading %s from %q: %w", linkdeps.LinkDepsFilename, depImportPath, err)
			}
			log.Debugf("Processing %s dependencies from %s[%s]...", linkdeps.LinkDepsFilename, depImportPath, archive)
			for _, tDep := range deps.Dependencies() {
				if _, found := imports.PackageFile[tDep]; !found {
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
					if _, found := imports.PackageFile[tDep]; !found {
						log.Debugf("Copying transitive %s dependency on %q inherited from %q via %q\n", linkdeps.LinkDepsFilename, tDep, depImportPath, dep)
						linkDeps.Add(tDep)
					}
				}

				if _, ok := imports.PackageFile[dep]; ok {
					// Already part of natural dependencies, nothing to do...
					continue
				}
				log.Debugf("Recording transitive dependency of %q: %q => %q\n", depImportPath, dep, archive)
				imports.PackageFile[dep] = archive
				regUpdated = true
			}
		}
	}

	if regUpdated {
		// Creating updated version of the importcfg file, with new dependencies
		if err := writeUpdatedImportConfig(imports, cmd.Flags.ImportCfg); err != nil {
			return fmt.Errorf("writing updated %q: %w", cmd.Flags.ImportCfg, err)
		}
	}

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
