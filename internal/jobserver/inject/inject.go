// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

// Package inject provides a NATS service that runs the instrumentation pipeline
// in the long-running job server process instead of in each toolexec subprocess.
// This eliminates per-process startup overhead (Go runtime init, config loading,
// YAML parsing, aspect construction) by caching these in the server.
package inject

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/DataDog/orchestrion/internal/injector"
	"github.com/DataDog/orchestrion/internal/injector/aspect"
	"github.com/DataDog/orchestrion/internal/injector/cache"
	"github.com/DataDog/orchestrion/internal/injector/config"
	"github.com/DataDog/orchestrion/internal/injector/typed"
	"github.com/DataDog/orchestrion/internal/jobserver/common"
	"github.com/DataDog/orchestrion/internal/toolexec/importcfg"
	"github.com/nats-io/nats.go"
	"github.com/rs/zerolog"
)

const injectSubject = "compile.inject"

// Request contains all data needed to run the instrumentation pipeline
// for a single package. All file paths are absolute on the shared filesystem.
type Request struct {
	ImportPath     string            `json:"importPath"`
	GoVersion      string            `json:"goVersion"`
	TestMain       bool              `json:"testMain"`
	PackageName    string            `json:"packageName"`
	GoFiles        []string          `json:"goFiles"`
	PackageFile    map[string]string `json:"packageFile"`
	ImportMap      map[string]string `json:"importMap,omitempty"`
	OutputDir      string            `json:"outputDir"`
	ConfigFiles    []string          `json:"configFiles,omitempty"`
	NeverWeave     bool              `json:"neverWeave,omitempty"`
	TracerInternal bool              `json:"tracerInternal,omitempty"` // only keep TracerInternal aspects
}

func (Request) Subject() string      { return injectSubject }
func (Request) ResponseIs(Response)  {}
func (r Request) ForeachSpanTag(set func(string, any)) {
	set("request.importPath", r.ImportPath)
	set("request.packageName", r.PackageName)
	set("request.goFiles.count", len(r.GoFiles))
}

// ModifiedFileEntry represents a single modified source file.
type ModifiedFileEntry struct {
	OriginalPath string          `json:"originalPath"`
	ModifiedPath string          `json:"modifiedPath"`
	References   map[string]bool `json:"references,omitempty"` // import path → is ImportStatement
}

// Response contains the results of the instrumentation pipeline.
type Response struct {
	ModifiedFiles []ModifiedFileEntry `json:"modifiedFiles,omitempty"`
	GoLang        string              `json:"goLang,omitempty"`
	LinkDeps      []string            `json:"linkDeps,omitempty"`
	Skipped       bool                `json:"skipped,omitempty"` // true if NeverWeave
}

type service struct {
	packageLoader config.PackageLoader

	// Cached aspects — loaded once, reused for all requests.
	aspectsMu     sync.Mutex
	cachedAspects []*aspect.Aspect
	configKey     string // the config files key used for cache invalidation
}

// Subscribe registers the inject service on the given NATS connection.
func Subscribe(ctx context.Context, conn *nats.Conn, pkgLoader config.PackageLoader) error {
	s := &service{packageLoader: pkgLoader}
	_, err := conn.Subscribe(injectSubject, common.HandleRequest(ctx, s.handle))
	return err
}

func (s *service) handle(ctx context.Context, req Request) (Response, error) {
	log := zerolog.Ctx(ctx)

	// Load and cache aspects.
	aspects, err := s.getAspects(ctx, req.ConfigFiles)
	if err != nil {
		return Response{}, fmt.Errorf("loading aspects: %w", err)
	}

	// Apply special-case behavior overrides (computed by the shim).
	if req.NeverWeave {
		return Response{Skipped: true}, nil
	}
	if req.TracerInternal {
		filtered := make([]*aspect.Aspect, 0, len(aspects))
		for _, a := range aspects {
			if a.TracerInternal {
				filtered = append(filtered, a)
			}
		}
		aspects = filtered
	}

	// Build the importcfg from the request data.
	imports := importcfg.ImportConfig{
		PackageFile: req.PackageFile,
		ImportMap:   req.ImportMap,
	}

	testMain := req.TestMain
	modifiedFilePath := func(file string) string {
		return filepath.Join(req.OutputDir, "orchestrion", "src", req.PackageName, filepath.Base(file))
	}

	// Compute cache key.
	importPaths := make([]string, 0, len(req.PackageFile))
	for k := range req.PackageFile {
		importPaths = append(importPaths, k)
	}
	cacheKey := cache.ComputeKey(req.ImportPath, req.GoVersion, testMain, req.GoFiles, req.ConfigFiles, importPaths)

	// Check persistent cache.
	if cached := cache.Lookup(cacheKey); cached != nil {
		log.Debug().Str("import-path", req.ImportPath).Msg("Server-side instrumentation cache hit")
		resp, err := applyCached(cached, modifiedFilePath)
		if err == nil {
			return resp, nil
		}
		log.Warn().Err(err).Msg("Failed to apply cached result, falling back to full instrumentation")
	}

	// Run the full instrumentation pipeline.
	inj := injector.Injector{
		RootConfig:   map[string]string{"httpmode": "wrap"},
		Lookup:       imports.Lookup,
		ImportPath:   req.ImportPath,
		TestMain:     testMain,
		ImportMap:    imports.PackageFile,
		GoVersion:    req.GoVersion,
		ModifiedFile: modifiedFilePath,
	}

	results, goLang, err := inj.InjectFiles(ctx, req.GoFiles, aspects)
	if err != nil {
		return Response{}, err
	}

	// Build response.
	resp := Response{
		GoLang: goLang.String(),
	}

	var allRefs typed.ReferenceMap
	for origPath, modFile := range results {
		refs := make(map[string]bool, modFile.References.Count())
		for importPath, kind := range modFile.References.Map() {
			refs[importPath] = bool(kind)
		}
		resp.ModifiedFiles = append(resp.ModifiedFiles, ModifiedFileEntry{
			OriginalPath: origPath,
			ModifiedPath: modFile.Filename,
			References:   refs,
		})
		allRefs.Merge(modFile.References)
	}

	// Collect link deps from references.
	for depImportPath, kind := range allRefs.Map() {
		if depImportPath == "unsafe" {
			continue
		}
		if _, satisfied := imports.PackageFile[depImportPath]; satisfied {
			continue
		}
		resp.LinkDeps = append(resp.LinkDeps, depImportPath)
		_ = kind // linkdeps are all recorded regardless of kind
	}

	// Store in persistent cache.
	storeCacheEntry(cacheKey, results, goLang)

	return resp, nil
}

// getAspects returns the cached aspects list, loading it on first call or when
// config files change.
func (s *service) getAspects(ctx context.Context, configFiles []string) ([]*aspect.Aspect, error) {
	key := strings.Join(configFiles, string(os.PathListSeparator))

	s.aspectsMu.Lock()
	defer s.aspectsMu.Unlock()

	if s.cachedAspects != nil && s.configKey == key {
		return s.cachedAspects, nil
	}

	var cfg config.Config
	var err error
	if len(configFiles) > 0 {
		cfg, err = config.LoadFromFiles(ctx, configFiles)
	} else {
		cfg, err = config.NewLoader(s.packageLoader, ".", false).Load(ctx)
	}
	if err != nil {
		return nil, err
	}

	s.cachedAspects = cfg.Aspects()
	s.configKey = key
	return s.cachedAspects, nil
}

func applyCached(entry *cache.Entry, modifiedFilePath func(string) string) (Response, error) {
	resp := Response{GoLang: entry.GoLang}
	for origPath, modFile := range entry.ModifiedFiles {
		outPath := modifiedFilePath(origPath)
		dir := filepath.Dir(outPath)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return Response{}, fmt.Errorf("mkdir %q: %w", dir, err)
		}
		if err := os.WriteFile(outPath, modFile.Content, 0o644); err != nil {
			return Response{}, fmt.Errorf("writing %q: %w", outPath, err)
		}
		resp.ModifiedFiles = append(resp.ModifiedFiles, ModifiedFileEntry{
			OriginalPath: origPath,
			ModifiedPath: outPath,
			References:   modFile.References,
		})
		for importPath := range modFile.References {
			resp.LinkDeps = append(resp.LinkDeps, importPath)
		}
	}
	return resp, nil
}

func storeCacheEntry(key cache.Key, results map[string]injector.InjectedFile, goLang interface{ String() string }) {
	if len(results) == 0 {
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
