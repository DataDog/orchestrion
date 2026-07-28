// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package pkgs

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"golang.org/x/tools/go/packages"
)

type overlayManifest struct {
	Replace map[string]string
}

func addTestVariantOverlay(ctx context.Context, config *packages.Config, virtualPath string, contents []byte) (func(), error) {
	if err := rejectModuleCacheOverlay(ctx, config, virtualPath); err != nil {
		return nil, err
	}

	dir, err := os.MkdirTemp("", "orchestrion-test-overlay-")
	if err != nil {
		return nil, fmt.Errorf("creating test variant overlay directory: %w", err)
	}
	cleanup := func() { _ = os.RemoveAll(dir) }
	failed := true
	defer func() {
		if failed {
			cleanup()
		}
	}()

	manifest := overlayManifest{Replace: make(map[string]string)}
	var callerOverlay string
	flags := make([]string, 0, len(config.BuildFlags))
	for _, flag := range config.BuildFlags {
		if value, found := strings.CutPrefix(flag, "-overlay="); found {
			callerOverlay = value
			continue
		}
		flags = append(flags, flag)
	}
	config.BuildFlags = flags

	if callerOverlay != "" {
		path := callerOverlay
		if !filepath.IsAbs(path) {
			path = filepath.Join(config.Dir, path)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("reading caller overlay %q: %w", path, err)
		}
		var caller overlayManifest
		if err := json.Unmarshal(data, &caller); err != nil {
			return nil, fmt.Errorf("parsing caller overlay %q: %w", path, err)
		}
		originalSources := make(map[string]string, len(caller.Replace))
		for source, replacement := range caller.Replace {
			if source == "" {
				return nil, fmt.Errorf("caller overlay %q contains an empty source path", path)
			}
			originalSource := source
			source = absoluteOverlayPath(config.Dir, source)
			if previous, exists := originalSources[source]; exists && previous != originalSource {
				return nil, fmt.Errorf("caller overlay %q contains duplicate paths %q and %q", path, previous, originalSource)
			}
			originalSources[source] = originalSource
			if replacement != "" {
				replacement = absoluteOverlayPath(config.Dir, replacement)
			}
			manifest.Replace[source] = replacement
		}
	}

	virtualPath = absoluteOverlayPath(config.Dir, virtualPath)
	if _, exists := manifest.Replace[virtualPath]; exists {
		return nil, fmt.Errorf("caller overlay already replaces Orchestrion test variant path %q", virtualPath)
	}
	backingPath := filepath.Join(dir, filepath.Base(virtualPath))
	if err := os.WriteFile(backingPath, contents, 0o644); err != nil {
		return nil, fmt.Errorf("writing test variant overlay source: %w", err)
	}
	manifest.Replace[virtualPath] = backingPath

	manifestPath := filepath.Join(dir, "overlay.json")
	file, err := os.Create(manifestPath)
	if err != nil {
		return nil, fmt.Errorf("creating combined overlay: %w", err)
	}
	if err := json.NewEncoder(file).Encode(manifest); err != nil {
		_ = file.Close()
		return nil, fmt.Errorf("writing combined overlay: %w", err)
	}
	if err := file.Close(); err != nil {
		return nil, fmt.Errorf("closing combined overlay: %w", err)
	}

	config.Overlay = nil
	config.BuildFlags = append(config.BuildFlags, "-overlay="+manifestPath)
	failed = false
	return cleanup, nil
}

func rejectModuleCacheOverlay(ctx context.Context, config *packages.Config, virtualPath string) error {
	cmd := exec.CommandContext(ctx, "go", "env", "GOMODCACHE")
	cmd.Dir = config.Dir
	cmd.Env = config.Env
	output, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("determining GOMODCACHE: %w", err)
	}
	moduleCache := strings.TrimSpace(string(output))
	if moduleCache == "" {
		return nil
	}
	inside, err := pathWithin(moduleCache, virtualPath)
	if err != nil {
		return err
	}
	if inside {
		return fmt.Errorf("cannot construct test variant overlay %q beneath GOMODCACHE %q; Go does not permit module-cache overlays", virtualPath, moduleCache)
	}
	return nil
}

func pathWithin(parent string, child string) (bool, error) {
	parent, err := filepath.Abs(parent)
	if err != nil {
		return false, err
	}
	child, err = filepath.Abs(child)
	if err != nil {
		return false, err
	}
	rel, err := filepath.Rel(parent, child)
	if err != nil {
		return false, err
	}
	return rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)), nil
}

func absoluteOverlayPath(dir string, path string) string {
	if filepath.IsAbs(path) {
		return filepath.Clean(path)
	}
	return filepath.Clean(filepath.Join(dir, path))
}
