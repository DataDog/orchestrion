// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package pkgs

import (
	"context"
	"fmt"
	"sync"

	"github.com/DataDog/orchestrion/internal/injector"
	"github.com/DataDog/orchestrion/internal/injector/aspect"
	"github.com/DataDog/orchestrion/internal/injector/config"
)

type (
	// MayModifyRequest asks whether any aspect of the injector configuration may apply to a package's
	// source files; so that compile commands can avoid loading the configuration themselves when
	// none can.
	MayModifyRequest struct {
		ConfigDir  string   `json:"configDir"`  // The directory the injector configuration is loaded from
		WorkDir    string   `json:"workDir"`    // The working directory of the build
		ImportPath string   `json:"importPath"` // The import path of the package being compiled
		Imports    []string `json:"imports"`    // The import paths listed in the package's importcfg
		TestMain   bool     `json:"testMain"`   // Whether the package is Go's generated test main
		Files      []string `json:"files"`      // The absolute paths of the package's Go source files
	}
	// MayModifyResponse tells whether any aspect may apply to a package. Its zero value, which is also
	// what a response without a result decodes to, means that some aspect may apply.
	MayModifyResponse struct {
		// NoAspectMayApply is true only if no aspect can match on any of the package's files.
		NoAspectMayApply bool `json:"noAspectMayApply"`
	}
)

func (MayModifyRequest) Subject() string              { return mayModifySubject }
func (MayModifyRequest) ResponseIs(MayModifyResponse) {}
func (r MayModifyRequest) ForeachSpanTag(set func(key string, value any)) {
	set("request.config-dir", r.ConfigDir)
	set("request.import-path", r.ImportPath)
}

func (s *service) mayModify(ctx context.Context, req MayModifyRequest) (MayModifyResponse, error) {
	aspects, err := s.configs.aspects(ctx, req.ConfigDir, s.packageLoader)
	if err != nil {
		return MayModifyResponse{}, err
	}

	importMap := make(map[string]string, len(req.Imports))
	for _, path := range req.Imports {
		importMap[path] = ""
	}
	inj := injector.Injector{
		ImportPath: req.ImportPath,
		ImportMap:  importMap,
		TestMain:   req.TestMain,
		WorkDir:    req.WorkDir,
	}
	return MayModifyResponse{NoAspectMayApply: !inj.MayModify(req.Files, aspects)}, nil
}

type (
	// configSnapshots holds the injector configuration loaded from each directory, for as long as the
	// files it was loaded from are unchanged.
	configSnapshots struct {
		mu    sync.Mutex
		byDir map[string]*configSnapshotSlot
	}
	configSnapshotSlot struct {
		mu       sync.Mutex
		snapshot *configSnapshot
	}
	configSnapshot struct {
		aspects []*aspect.Aspect
		files   config.FileDigests
	}
)

// aspects returns the aspects of the injector configuration loaded from dir. The configuration is
// cached, and re-loaded if any of the files it was loaded from has changed (or if any of the files
// that were looked for has been created) since; so the result is the same as loading it anew.
func (c *configSnapshots) aspects(ctx context.Context, dir string, pkgLoader config.PackageLoader) ([]*aspect.Aspect, error) {
	c.mu.Lock()
	if c.byDir == nil {
		c.byDir = make(map[string]*configSnapshotSlot)
	}
	slot := c.byDir[dir]
	if slot == nil {
		slot = &configSnapshotSlot{}
		c.byDir[dir] = slot
	}
	c.mu.Unlock()

	slot.mu.Lock()
	snapshot := slot.snapshot
	slot.mu.Unlock()
	if snapshot != nil && snapshot.files.Unchanged() {
		return snapshot.aspects, nil
	}

	slot.mu.Lock()
	defer slot.mu.Unlock()
	if slot.snapshot != snapshot && slot.snapshot.files.Unchanged() {
		// Another request has re-loaded the configuration in the meantime.
		return slot.snapshot.aspects, nil
	}

	loader := config.NewLoader(pkgLoader, dir, false)
	cfg, err := loader.Load(ctx)
	if err != nil {
		return nil, fmt.Errorf("loading injector configuration: %w", err)
	}
	slot.snapshot = &configSnapshot{aspects: cfg.Aspects(), files: loader.Files()}
	return slot.snapshot.aspects, nil
}
