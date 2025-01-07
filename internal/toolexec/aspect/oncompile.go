// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package aspect

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"

	"github.com/DataDog/orchestrion/internal/files"
	"github.com/DataDog/orchestrion/internal/goenv"
	"github.com/DataDog/orchestrion/internal/injector"
	"github.com/DataDog/orchestrion/internal/injector/aspect"
	"github.com/DataDog/orchestrion/internal/injector/config"
	"github.com/DataDog/orchestrion/internal/injector/typed"
	"github.com/DataDog/orchestrion/internal/jobserver/client"
	"github.com/DataDog/orchestrion/internal/jobserver/nbt"
	"github.com/DataDog/orchestrion/internal/toolexec/aspect/linkdeps"
	"github.com/DataDog/orchestrion/internal/toolexec/importcfg"
	"github.com/DataDog/orchestrion/internal/toolexec/proxy"
	"github.com/rs/zerolog"
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

func (w Weaver) OnCompile(ctx context.Context, cmd *proxy.CompileCommand) (err error) {
	log := zerolog.Ctx(ctx).With().Str("phase", "compile").Logger()
	ctx = log.WithContext(ctx)

	if js, err := client.FromEnvironment(ctx, cmd.WorkDir); err != nil {
		log.Debug().Str("work-dir", cmd.WorkDir).Err(err).Msg("Failed to obtain job server client")
	} else {
		res, err := client.Request[nbt.StartRequest, *nbt.StartResponse](ctx, js, nbt.StartRequest{ImportPath: w.ImportPath})
		if err != nil {
			js.Close()
			return err
		}
		if res.ArchivePath != "" {
			defer js.Close()
			log.Debug().Str("archive", res.ArchivePath).Msg("Using pre-built archive")
			if err := files.LinkOrCopy(ctx, res.ArchivePath, cmd.Flags.Output); err != nil {
				return err
			}
			return proxy.ErrSkipCommand
		}

		cmd.OnClose(func(exitErr error) error {
			defer js.Close()

			var error *string
			if err != nil {
				msg := errors.Join(exitErr, err).Error()
				error = &msg
			}
			_, err := client.Request[nbt.FinishRequest, *nbt.FinishResponse](ctx, js, nbt.FinishRequest{
				ImportPath:  w.ImportPath,
				FinishToken: res.FinishToken,
				ArchivePath: cmd.Flags.Output,
				Error:       error,
			})
			return err
		})
	}

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
		err = writeLinkDeps(log, cmd, &linkDeps, orchestrionDir)
	}()

	goMod, err := goenv.GOMOD(".")
	if err != nil {
		return fmt.Errorf("go env GOMOD: %w", err)
	}
	goModDir := filepath.Dir(goMod)
	log.Trace().Str("module.dir", goModDir).Msg("Identified module directory")

	cfg, err := config.NewLoader(goModDir, false).Load()
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
			log.Debug().Str("import-path", w.ImportPath).Msg("Not weaving aspects to prevent circular instrumentation")
			return nil

		case weaveTracerInternal:
			log.Debug().Str("import-path", w.ImportPath).Msg("Enabling tracer-internal mode")
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
	results, goLang, err := injector.InjectFiles(ctx, goFiles, aspects)
	if err != nil {
		return err
	}

	if err := cmd.SetLang(goLang); err != nil {
		return err
	}

	references := typed.ReferenceMap{}
	for gofile, modFile := range results {
		log.Debug().Str("original", gofile).Str("updated", modFile.Filename).Msg("Replacing argument for modified source code")
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

		log.Debug().Stringer("kind", kind).Str("import-path", depImportPath).Msg("Recording synthetic " + linkdeps.Filename + " dependency")
		linkDeps.Add(depImportPath)

		if kind != typed.ImportStatement {
			// We cannot attempt to resolve link-time dependencies (relocation targets), as these are
			// typically used to avoid creating dependency cycles. Corrollary to this, the `link.deps`
			// file will not contain transitive closures for these packages, so we need to resolve these
			// at link-time.
			continue
		}

		// Imported packages need to be provided in the compilation's importcfg file
		deps, err := resolvePackageFiles(ctx, depImportPath, cmd.WorkDir)
		if err != nil {
			return fmt.Errorf("resolving woven dependency on %s: %w", depImportPath, err)
		}
		for dep, archive := range deps {
			deps, err := linkdeps.FromArchive(archive)
			if err != nil {
				return fmt.Errorf("reading %s from %s[%s]: %w", linkdeps.Filename, dep, archive, err)
			}
			log.Debug().Str("import-path", dep).Msg("Processing " + linkdeps.Filename + " dependencies")
			for _, tDep := range deps.Dependencies() {
				if _, found := imports.PackageFile[tDep]; !found {
					log.Debug().Str("import-path", dep).Str("transitive", tDep).Str("inherited-from", depImportPath).Msg("Copying transitive " + linkdeps.Filename + " dependency")
					linkDeps.Add(tDep)
				}
			}

			if _, ok := imports.PackageFile[dep]; ok {
				// Already part of natural dependencies, nothing to do...
				continue
			}
			log.Debug().Str("import-path", dep).Str("inherited-from", depImportPath).Str("archive", archive).Msg("Recording transitive dependency")
			imports.PackageFile[dep] = archive
			regUpdated = true
		}
	}

	if regUpdated {
		// Creating updated version of the importcfg file, with new dependencies
		if err := writeUpdatedImportConfig(log, imports, cmd.Flags.ImportCfg); err != nil {
			return fmt.Errorf("writing updated %q: %w", cmd.Flags.ImportCfg, err)
		}
	}

	return nil
}

func writeUpdatedImportConfig(log zerolog.Logger, reg importcfg.ImportConfig, filename string) (err error) {
	const dotOriginal = ".original"

	log.Trace().Str("path", filename).Msg("Backing up original file")
	if err := os.Rename(filename, filename+dotOriginal); err != nil {
		return fmt.Errorf("renaming to %q: %w", filepath.Base(filename)+dotOriginal, err)
	}

	log.Debug().Str("path", filename).Msg("Writing updated file")
	if err := reg.WriteFile(filename); err != nil {
		return fmt.Errorf("writing: %w", err)
	}

	return nil
}

// writeLinkDeps writes the [linkdeps.Filename] file into the orchestrionDir,
// and registers it to be packed into the output archive. Does nothing if the
// provided [linkdeps.LinkDeps] is empty.
func writeLinkDeps(log zerolog.Logger, cmd *proxy.CompileCommand, linkDeps *linkdeps.LinkDeps, orchestrionDir string) error {
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

	cmd.OnClose(func(err error) error {
		if err != nil {
			// Don't try to add to the archive if the compilation failed!
			return nil
		}

		log.Debug().Str("archive", cmd.Flags.Output).Array(linkdeps.Filename, linkDeps).Msg("Adding " + linkdeps.Filename + " file in archive")
		child := exec.Command("go", "tool", "pack", "r", cmd.Flags.Output, linkDepsFile)
		if err := child.Run(); err != nil {
			return fmt.Errorf("running %q: %w", child.Args, err)
		}
		return nil
	})

	return nil
}
