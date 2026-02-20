// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package aspect

import (
	"bytes"
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
	injcontext "github.com/DataDog/orchestrion/internal/injector/aspect/context"
	"github.com/DataDog/orchestrion/internal/injector/cache"
	"github.com/DataDog/orchestrion/internal/injector/config"
	"github.com/DataDog/orchestrion/internal/injector/typed"
	"github.com/DataDog/orchestrion/internal/jobserver/client"
	"github.com/DataDog/orchestrion/internal/jobserver/inject"
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
		imports = *cmd.Imports
	} else {
		var err error
		imports, err = importcfg.ParseFile(ctx, cmd.Flags.ImportCfg)
		if err != nil {
			return fmt.Errorf("parsing %q: %w", cmd.Flags.ImportCfg, err)
		}
	}

	// Determine special-case behavior for this package.
	specialBehavior, _ := FindBehaviorOverride(w.ImportPath)

	if specialBehavior == NeverWeave {
		log.Debug().Str("import-path", w.ImportPath).Msg("Not weaving aspects to prevent circular instrumentation")
		return nil
	}

	// Early skip: if we have the pre-computed set of import paths that aspects
	// care about, check if this package imports any of them. If not, skip the
	// inject NATS call entirely (~95% of packages in a typical build).
	if canSkipInstrumentation(w.ImportPath, imports.PackageFile, cmd.GoFiles()) {
		log.Debug().Str("import-path", w.ImportPath).Msg("Skipping instrumentation (no eligible imports)")
		return nil
	}

	// Try the server-side inject service first (eliminates per-process config
	// loading, YAML parsing, and aspect construction overhead).
	js, err := client.FromEnvironment(ctx, cmd.WorkDir)
	if err != nil {
		return err
	}

	var configFiles []string
	if cf := os.Getenv(config.EnvVarConfigFiles); cf != "" {
		configFiles = strings.Split(cf, string(os.PathListSeparator))
	}

	resp, err := client.Request(ctx, js, inject.Request{
		ImportPath:     w.ImportPath,
		GoVersion:      cmd.Flags.Lang,
		TestMain:       cmd.TestMain() && strings.HasSuffix(w.ImportPath, ".test"),
		PackageName:    cmd.Flags.Package,
		GoFiles:        cmd.GoFiles(),
		PackageFile:    imports.PackageFile,
		ImportMap:      imports.ImportMap,
		OutputDir:      filepath.Dir(cmd.Flags.Output),
		ConfigFiles:    configFiles,
		NeverWeave:     specialBehavior == NeverWeave,
		TracerInternal: specialBehavior == WeaveTracerInternal,
	})

	if err != nil {
		// Fallback to local execution if the inject service is unavailable.
		log.Debug().Err(err).Msg("Server-side inject failed, falling back to local execution")
		return w.onCompileLocal(ctx, log, cmd, &imports, configFiles, specialBehavior)
	}

	if resp.Skipped {
		log.Debug().Str("import-path", w.ImportPath).Msg("Not weaving aspects (server-side skip)")
		return nil
	}

	// Apply the server's response.
	if resp.GoLang != "" {
		goLang, _ := injcontext.ParseGoLangVersion(resp.GoLang)
		if err := cmd.SetLang(goLang); err != nil {
			return err
		}
	}

	for _, entry := range resp.ModifiedFiles {
		log.Debug().Str("original", entry.OriginalPath).Str("updated", entry.ModifiedPath).Msg("Replacing argument for modified source code")
		if err := cmd.ReplaceParam(entry.OriginalPath, entry.ModifiedPath); err != nil {
			return fmt.Errorf("replacing %q with %q: %w", entry.OriginalPath, entry.ModifiedPath, err)
		}
	}

	// Process link deps and resolve new dependencies.
	if len(resp.LinkDeps) == 0 {
		return nil
	}

	var regUpdated bool
	for _, depImportPath := range resp.LinkDeps {
		if depImportPath == "unsafe" {
			continue
		}
		if _, satisfied := imports.PackageFile[depImportPath]; satisfied {
			continue
		}

		log.Debug().Str("import-path", depImportPath).Msg("Recording synthetic " + linkdeps.Filename + " dependency")
		cmd.LinkDeps.Add(depImportPath)

		// Determine the reference kind from the response entry.
		isImportStmt := false
		for _, entry := range resp.ModifiedFiles {
			if v, ok := entry.References[depImportPath]; ok && v {
				isImportStmt = true
				break
			}
		}

		if !isImportStmt && cmd.Flags.Package != "main" {
			continue
		}

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
					cmd.LinkDeps.Add(tDep)
				}
			}

			if _, ok := imports.PackageFile[dep]; ok {
				continue
			}
			imports.PackageFile[dep] = archive
			regUpdated = true
		}
	}

	if regUpdated {
		if err := writeUpdatedImportConfig(log, imports, cmd.Flags.ImportCfg); err != nil {
			return fmt.Errorf("writing updated %q: %w", cmd.Flags.ImportCfg, err)
		}
	}

	return nil
}

// onCompileLocal is the fallback path that runs the full instrumentation
// pipeline locally in the toolexec process when the server-side inject
// service is unavailable.
func (w Weaver) onCompileLocal(ctx context.Context, log zerolog.Logger, cmd *proxy.CompileCommand, imports *importcfg.ImportConfig, configFiles []string, specialBehavior BehaviorOverride) error {
	var cfg config.Config
	var err error
	if len(configFiles) > 0 {
		cfg, err = config.LoadFromFiles(ctx, configFiles)
	} else {
		goMod, goModErr := goenv.GOMOD(".")
		if goModErr != nil {
			return fmt.Errorf("go env GOMOD: %w", goModErr)
		}
		goModDir := filepath.Dir(goMod)
		js, jsErr := client.FromEnvironment(ctx, cmd.WorkDir)
		if jsErr != nil {
			return jsErr
		}
		cfg, err = config.NewLoader(packageLoader(js), goModDir, false).Load(ctx)
	}
	if err != nil {
		return fmt.Errorf("loading injector configuration: %w", err)
	}

	aspects := cfg.Aspects()
	switch specialBehavior {
	case NeverWeave:
		return nil
	case WeaveTracerInternal:
		aspects = slices.DeleteFunc(aspects, func(a *aspect.Aspect) bool {
			return !a.TracerInternal
		})
	}

	testMain := cmd.TestMain() && strings.HasSuffix(w.ImportPath, ".test")
	modifiedFilePath := func(file string) string {
		return filepath.Join(filepath.Dir(cmd.Flags.Output), OrchestrionDirPathElement, cmd.Flags.Package, filepath.Base(file))
	}

	goFiles := cmd.GoFiles()
	importPaths := make([]string, 0, len(imports.PackageFile))
	for k := range imports.PackageFile {
		importPaths = append(importPaths, k)
	}
	cacheKey := cache.ComputeKey(w.ImportPath, cmd.Flags.Lang, testMain, goFiles, configFiles, importPaths)

	references, err := w.injectOrRestoreFromCache(ctx, log, cmd, cacheKey, goFiles, aspects, testMain, modifiedFilePath, imports)
	if err != nil {
		return err
	}

	if references.Count() == 0 {
		return nil
	}

	var regUpdated bool
	for depImportPath, kind := range references.Map() {
		if depImportPath == "unsafe" {
			continue
		}
		if _, satisfied := imports.PackageFile[depImportPath]; satisfied {
			continue
		}

		cmd.LinkDeps.Add(depImportPath)

		if kind != typed.ImportStatement && cmd.Flags.Package != "main" {
			continue
		}

		deps, err := resolvePackageFiles(ctx, depImportPath, cmd.WorkDir)
		if err != nil {
			return fmt.Errorf("resolving woven dependency on %s: %w", depImportPath, err)
		}
		for dep, archive := range deps {
			deps, err := linkdeps.FromArchive(ctx, archive)
			if err != nil {
				return fmt.Errorf("reading %s from %s[%s]: %w", linkdeps.Filename, dep, archive, err)
			}
			for _, tDep := range deps.Dependencies() {
				if _, found := imports.PackageFile[tDep]; !found {
					cmd.LinkDeps.Add(tDep)
				}
			}

			if _, ok := imports.PackageFile[dep]; ok {
				continue
			}
			imports.PackageFile[dep] = archive
			regUpdated = true
		}
	}

	if regUpdated {
		if err := writeUpdatedImportConfig(log, *imports, cmd.Flags.ImportCfg); err != nil {
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

// canSkipInstrumentation returns true if the package definitely can't match any
// aspect, based on the pre-computed eligible imports set. This avoids the inject
// NATS round-trip for ~95% of packages in a typical build.
func canSkipInstrumentation(importPath string, packageFile map[string]string, goFiles []string) bool {
	eligibleStr := os.Getenv(config.EnvVarEligibleImports)
	if eligibleStr == "" {
		return false // no eligible imports info available, can't skip
	}

	eligible := strings.Split(eligibleStr, string(os.PathListSeparator))

	// Check if the package's import map intersects with the eligible set.
	for _, path := range eligible {
		if _, ok := packageFile[path]; ok {
			return false // this package imports something aspects care about
		}
		if path == importPath {
			return false // the package IS one that aspects target
		}
	}

	// No eligible imports found. As a safety check, scan source files for
	// orchestrion directive comments which can match any package.
	for _, f := range goFiles {
		if fileContainsDirective(f) {
			return false
		}
	}

	return true
}

// fileContainsDirective does a cheap byte scan for orchestrion directive
// comments. This is much faster than the full inject pipeline (~0.01ms vs
// ~1-5ms per file).
func fileContainsDirective(path string) bool {
	data, err := os.ReadFile(path)
	if err != nil {
		return true // can't read → don't skip (conservative)
	}
	return bytes.Contains(data, []byte("//orchestrion:")) ||
		bytes.Contains(data, []byte("//dd:orchestrion"))
}

func packageLoader(js *client.Client) config.PackageLoader {
	return func(ctx context.Context, dir string, patterns ...string) ([]*packages.Package, error) {
		return client.Request(ctx, js, pkgs.LoadRequest{Dir: dir, Patterns: patterns})
	}
}

// injectOrRestoreFromCache either restores a cached instrumentation result or
// runs the full InjectFiles pipeline and caches the result for future builds.
func (w Weaver) injectOrRestoreFromCache(
	ctx context.Context,
	log zerolog.Logger,
	cmd *proxy.CompileCommand,
	cacheKey cache.Key,
	goFiles []string,
	aspects []*aspect.Aspect,
	testMain bool,
	modifiedFilePath func(string) string,
	imports *importcfg.ImportConfig,
) (typed.ReferenceMap, error) {
	references := typed.ReferenceMap{}

	// Check the persistent cache for a hit.
	if cached := cache.Lookup(cacheKey); cached != nil {
		log.Debug().Str("import-path", w.ImportPath).Msg("Instrumentation cache hit")
		if err := applyCachedResult(cmd, cached, modifiedFilePath, &references); err != nil {
			log.Warn().Err(err).Msg("Failed to apply cached result, falling back to full instrumentation")
			references = typed.ReferenceMap{}
		} else {
			return references, nil
		}
	}

	// Cache miss: run full instrumentation pipeline.
	inj := injector.Injector{
		RootConfig:   map[string]string{"httpmode": "wrap"},
		Lookup:       imports.Lookup,
		ImportPath:   w.ImportPath,
		TestMain:     testMain,
		ImportMap:    imports.PackageFile,
		GoVersion:    cmd.Flags.Lang,
		ModifiedFile: modifiedFilePath,
	}

	results, goLang, err := inj.InjectFiles(ctx, goFiles, aspects)
	if err != nil {
		return references, err
	}

	if err := cmd.SetLang(goLang); err != nil {
		return references, err
	}

	for gofile, modFile := range results {
		log.Debug().Str("original", gofile).Str("updated", modFile.Filename).Msg("Replacing argument for modified source code")
		if err := cmd.ReplaceParam(gofile, modFile.Filename); err != nil {
			return references, fmt.Errorf("replacing %q with %q: %w", gofile, modFile.Filename, err)
		}
		references.Merge(modFile.References)
	}

	// Store result in cache for future builds.
	storeCacheEntry(cacheKey, results, goLang)
	return references, nil
}

// applyCachedResult writes cached modified files to disk and populates the
// references map from cached data.
func applyCachedResult(
	cmd *proxy.CompileCommand,
	entry *cache.Entry,
	modifiedFilePath func(string) string,
	references *typed.ReferenceMap,
) error {
	if entry.GoLang != "" {
		goLang, _ := injcontext.ParseGoLangVersion(entry.GoLang)
		if err := cmd.SetLang(goLang); err != nil {
			return err
		}
	}

	for origPath, modFile := range entry.ModifiedFiles {
		outPath := modifiedFilePath(origPath)
		dir := filepath.Dir(outPath)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return fmt.Errorf("mkdir %q: %w", dir, err)
		}
		if err := os.WriteFile(outPath, modFile.Content, 0o644); err != nil {
			return fmt.Errorf("writing cached file %q: %w", outPath, err)
		}
		if err := cmd.ReplaceParam(origPath, outPath); err != nil {
			return fmt.Errorf("replacing %q with %q: %w", origPath, outPath, err)
		}
		// Reconstruct the ReferenceMap refs from cached data.
		for importPath, isImport := range modFile.References {
			references.AddRef(importPath, typed.ReferenceKind(isImport))
		}
	}
	return nil
}

// storeCacheEntry stores the instrumentation result in the persistent cache.
func storeCacheEntry(key cache.Key, results map[string]injector.InjectedFile, goLang injcontext.GoLangVersion) {
	if len(results) == 0 {
		// Nothing was modified, store an empty entry so we skip next time too.
		cache.Store(key, &cache.Entry{})
		return
	}

	entry := &cache.Entry{
		GoLang:        goLang.String(),
		ModifiedFiles: make(map[string]cache.ModifiedFile, len(results)),
	}
	for origPath, modFile := range results {
		content, err := os.ReadFile(modFile.Filename)
		if err != nil {
			// Can't cache if we can't read the file.
			return
		}
		refs := make(map[string]bool, modFile.References.Count())
		for importPath, kind := range modFile.References.Map() {
			refs[importPath] = bool(kind)
		}
		entry.ModifiedFiles[origPath] = cache.ModifiedFile{
			Content:    content,
			References: refs,
		}
	}

	cache.Store(key, entry)
}
