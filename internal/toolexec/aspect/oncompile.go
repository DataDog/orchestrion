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
	"slices"
	"strings"

	"github.com/DataDog/orchestrion/internal/injector"
	"github.com/DataDog/orchestrion/internal/injector/aspect"
	"github.com/DataDog/orchestrion/internal/injector/config"
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

func (w Weaver) OnCompile(cmd *proxy.CompileCommand) (err error) {
	log.SetContext("PHASE", "compile")
	defer log.SetContext("PHASE", "")

	imports, err := importcfg.ParseFile(cmd.Flags.ImportCfg)
	if err != nil {
		return fmt.Errorf("parsing %q: %w", cmd.Flags.ImportCfg, err)
	}

	linkDeps, err := linkdeps.FromImportConfig(&imports)
	if err != nil {
		return fmt.Errorf("reading %s closure from %s: %w", linkdeps.Filename, cmd.Flags.ImportCfg, err)
	}

	orchestrionDir := filepath.Join(filepath.Dir(cmd.Flags.Output), "orchestrion")

	// Ensure we correctly register the [linkdeps.Filename] into the output
	// archive upon returning, even if we made no changes. The contract is that
	// an archive's [linkdeps.Filename] must represent all transitive link-time
	// dependencies.
	defer func() {
		if err != nil {
			return
		}
		err = writeLinkDeps(cmd, &linkDeps, orchestrionDir)
	}()

	cfg, err := config.NewLoader(".", false).Load()
	if err != nil {
		return fmt.Errorf("loading injector configuration: %w", err)
	}

	aspects := cfg.Aspects()
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
			aspects = slices.DeleteFunc(aspects, func(a *aspect.Aspect) bool {
				return !a.TracerInternal
			})

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
		RootConfig: map[string]string{"httpmode": "wrap"},
		Lookup:     imports.Lookup,
		ImportPath: w.ImportPath,
		TestMain:   cmd.TestMain() && strings.HasSuffix(w.ImportPath, ".test"),
		ImportMap:  imports.PackageFile,
		GoVersion:  cmd.Flags.Lang,
		ModifiedFile: func(file string) string {
			return filepath.Join(orchestrionDir, "src", cmd.Flags.Package, filepath.Base(file))
		},
	}

	goFiles := cmd.GoFiles()
	results, goLang, err := injector.InjectFiles(goFiles, aspects)
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

		if _, satisfied := imports.PackageFile[depImportPath]; satisfied {
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
					return fmt.Errorf("reading %s from %s[%s]: %w", linkdeps.Filename, dep, archive, err)
				}
				log.Debugf("Processing %s dependencies from %s...\n", linkdeps.Filename, dep)
				for _, tDep := range deps.Dependencies() {
					if _, found := imports.PackageFile[tDep]; !found {
						log.Debugf("Copying transitive %s dependency on %q inherited from %q via %q\n", linkdeps.Filename, tDep, depImportPath, dep)
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

// writeLinkDeps writes the [linkdeps.Filename] file into the orchestrionDir,
// and registers it to be packed into the output archive. Does nothing if the
// provided [linkdeps.LinkDeps] is empty.
func writeLinkDeps(cmd *proxy.CompileCommand, linkDeps *linkdeps.LinkDeps, orchestrionDir string) error {
	if linkDeps.Empty() {
		// Nothing to do...
		return nil
	}

	// Write the link.deps file and add it to the output object once the compilation has completed.
	if err := os.MkdirAll(orchestrionDir, 0o755); err != nil {
		return fmt.Errorf("making directory %s: %w", orchestrionDir, err)
	}

	linkDepsFile := filepath.Join(orchestrionDir, linkdeps.Filename)
	if err := linkDeps.WriteFile(linkDepsFile); err != nil {
		return fmt.Errorf("writing %s file: %w", linkdeps.Filename, err)
	}

	cmd.OnClose(func() error {
		log.Debugf("Adding %s file into %q\n", linkdeps.Filename, cmd.Flags.Output)
		child := exec.Command("go", "tool", "pack", "r", cmd.Flags.Output, linkDepsFile)
		if err := child.Run(); err != nil {
			return fmt.Errorf("running %q: %w", child.Args, err)
		}
		return nil
	})

	return nil
}
