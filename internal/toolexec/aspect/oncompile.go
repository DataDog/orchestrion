// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package aspect

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/DataDog/dd-trace-go/v2/ddtrace/tracer"
	"github.com/DataDog/orchestrion/internal/goenv"
	"github.com/DataDog/orchestrion/internal/injector"
	"github.com/DataDog/orchestrion/internal/injector/aspect"
	"github.com/DataDog/orchestrion/internal/injector/config"
	"github.com/DataDog/orchestrion/internal/injector/typed"
	"github.com/DataDog/orchestrion/internal/jobserver/client"
	"github.com/DataDog/orchestrion/internal/jobserver/pkgs"
	"github.com/DataDog/orchestrion/internal/toolexec/aspect/linkdeps"
	"github.com/DataDog/orchestrion/internal/toolexec/importcfg"
	"github.com/DataDog/orchestrion/internal/toolexec/proxy"
	"github.com/rs/zerolog"
	"golang.org/x/tools/go/packages"
)

// OrchestrionDirPathElement is the prefix for orchestrion source files in the build output directory.
var OrchestrionDirPathElement = filepath.Join("orchestrion", "src")

func (w Weaver) OnCompile(ctx context.Context, cmd *proxy.CompileCommand) (resErr error) {
	span, ctx := tracer.StartSpanFromContext(ctx, "Weaver.OnCompile",
		tracer.ResourceName(w.ImportPath),
	)
	defer func() { span.Finish(tracer.WithError(resErr)) }()

	log := zerolog.Ctx(ctx).With().Str("phase", "compile").Str("import-path", w.ImportPath).Logger()
	ctx = log.WithContext(ctx)

	var imports importcfg.ImportConfig
	if cmd.Imports != nil {
		// Reuse the importcfg already parsed during parseCompileCommand.
		imports = *cmd.Imports
	} else {
		var err error
		imports, err = importcfg.ParseFile(ctx, cmd.Flags.ImportCfg)
		if err != nil {
			return fmt.Errorf("parsing %q: %w", cmd.Flags.ImportCfg, err)
		}
	}

	// Fast path: use pre-resolved config files from parent process (avoids
	// ~20 NATS round trips for package resolution per toolexec invocation).
	var cfg config.Config
	if configFiles := os.Getenv(config.EnvVarConfigFiles); configFiles != "" {
		files := strings.Split(configFiles, string(os.PathListSeparator))
		cfg, resErr = config.LoadFromFiles(ctx, files)
		if resErr != nil {
			return fmt.Errorf("loading injector configuration from pre-resolved files: %w", resErr)
		}
	} else {
		// Slow path: full config resolution via NATS.
		goMod, err := goenv.GOMOD(".")
		if err != nil {
			return fmt.Errorf("go env GOMOD: %w", err)
		}
		goModDir := filepath.Dir(goMod)
		log.Trace().Str("module.dir", goModDir).Msg("Identified module directory")

		js, err := client.FromEnvironment(ctx, cmd.WorkDir)
		if err != nil {
			return err
		}
		pkgLoader := packageLoader(js)

		cfg, resErr = config.NewLoader(pkgLoader, goModDir, false).Load(ctx)
		if resErr != nil {
			return fmt.Errorf("loading injector configuration: %w", resErr)
		}
	}

	aspects := cfg.Aspects()
	specialBehavior, isSpecial := FindBehaviorOverride(w.ImportPath)
	if isSpecial {
		switch specialBehavior {
		case NeverWeave:
			log.Debug().Str("import-path", w.ImportPath).Msg("Not weaving aspects to prevent circular instrumentation")
			return nil

		case WeaveTracerInternal:
			log.Debug().Str("import-path", w.ImportPath).Msg("Enabling tracer-internal mode")
			aspects = slices.DeleteFunc(aspects, func(a *aspect.Aspect) bool {
				return !a.TracerInternal
			})

		case NoOverride:
			// No-op

		default:
			// Unreachable
			panic(fmt.Sprintf("un-handled behavior override: %d", specialBehavior))
		}
	}

	injector := injector.Injector{
		RootConfig: map[string]string{"httpmode": "wrap"},
		Lookup:     imports.Lookup,
		ImportPath: w.ImportPath,
		TestMain:   cmd.TestMain() && strings.HasSuffix(w.ImportPath, ".test"),
		ImportMap:  imports.PackageFile,
		GoVersion:  cmd.Flags.Lang,
		ModifiedFile: func(file string) string {
			return filepath.Join(filepath.Dir(cmd.Flags.Output), OrchestrionDirPathElement, cmd.Flags.Package, filepath.Base(file))
		},
	}

	goFiles := cmd.GoFiles()
	results, goLang, resErr := injector.InjectFiles(ctx, goFiles, aspects)
	if resErr != nil {
		return resErr
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
		cmd.LinkDeps.Add(depImportPath)

		if kind != typed.ImportStatement && cmd.Flags.Package != "main" {
			// We cannot attempt to resolve link-time dependencies (relocation targets), as these are
			// typically used to avoid creating dependency cycles. Corollary to this, the `link.deps`
			// file will not contain transitive closures for these packages, so we need to resolve these
			// at link-time. If the package being built is "main", then we can ignore this, as we are at
			// the top-level of a dependency tree anyway, and if we cannot resolve a dependency, then we
			// will not be able to link the final binary.
			continue
		}

		// Imported packages need to be provided in the compilation's importcfg file
		deps, err := resolvePackageFiles(ctx, depImportPath, cmd.WorkDir)
		if err != nil {
			return fmt.Errorf("resolving woven dependency on %s: %w", depImportPath, err)
		}
		for dep, archive := range deps {
			deps, err := linkdeps.FromArchive(ctx, archive)
			if err != nil {
				return fmt.Errorf("reading %s from %s[%s]: %w", linkdeps.Filename, dep, archive, err)
			}
			log.Trace().Str("import-path", dep).Msg("Processing " + linkdeps.Filename + " dependencies")
			for _, tDep := range deps.Dependencies() {
				if _, found := imports.PackageFile[tDep]; !found {
					log.Trace().Str("import-path", dep).Str("transitive", tDep).Str("inherited-from", depImportPath).Msg("Copying transitive " + linkdeps.Filename + " dependency")
					cmd.LinkDeps.Add(tDep)
				}
			}

			if _, ok := imports.PackageFile[dep]; ok {
				// Already part of natural dependencies, nothing to do...
				continue
			}
			log.Trace().Str("import-path", dep).Str("inherited-from", depImportPath).Str("archive", archive).Msg("Recording transitive dependency")
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

func packageLoader(js *client.Client) config.PackageLoader {
	return func(ctx context.Context, dir string, patterns ...string) ([]*packages.Package, error) {
		return client.Request(ctx, js, pkgs.LoadRequest{Dir: dir, Patterns: patterns})
	}
}
