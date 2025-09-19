// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package goenv

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"sync"

	"golang.org/x/tools/go/packages"
)

var (
	// ErrNoGoMod is returned when no GOMOD value could be identified.
	ErrNoGoMod = errors.New("`go env GOMOD` returned a blank string")

	// ErrNoModulePath is returned when no module path could be identified.
	ErrNoModulePath = errors.New("no module path found")
)

// Module represents basic information about a Go module.
type Module struct {
	Path string `json:"Path"`
	Dir  string `json:"Dir"`
}

var (
	muCache sync.RWMutex
	// Cache for module path lookups to avoid repeated calls to go list.
	modulePathCache = make(map[string]string)
)

// GOMOD returns the current GOMOD environment variable (from running `go env GOMOD`).
func GOMOD(dir string) (string, error) {
	cmd := exec.Command("go", "env", "GOMOD")
	cmd.Dir = dir
	var stdout bytes.Buffer
	cmd.Stdout = &stdout
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("running %q: %w", cmd.Args, err)
	}
	if goMod := strings.TrimSpace(stdout.String()); goMod != "" && goMod != os.DevNull {
		return goMod, nil
	}

	wd, _ := os.Getwd()
	return "", fmt.Errorf("in %q: %w", wd, ErrNoGoMod)
}

// modulePath returns the module path of the current module using go/packages API.
// Results are cached to avoid repeated package loading calls.
func modulePath(ctx context.Context, dir string) (string, error) {
	muCache.RLock()
	cached, exists := modulePathCache[dir]
	muCache.RUnlock()
	if exists {
		return cached, nil
	}

	cfg := &packages.Config{
		Context: ctx,
		Dir:     dir,
		Mode:    packages.NeedModule,
	}

	pkgs, err := packages.Load(cfg, ".")
	if err != nil {
		return "", fmt.Errorf("loading package in %q: %w", dir, err)
	}

	if len(pkgs) == 0 || pkgs[0].Module == nil {
		return "", fmt.Errorf("in %q: %w", dir, ErrNoModulePath)
	}

	modulePath := pkgs[0].Module.Path
	if modulePath == "" {
		return "", fmt.Errorf("in %q: %w", dir, ErrNoModulePath)
	}

	muCache.Lock()
	defer muCache.Unlock()
	modulePathCache[dir] = modulePath

	return modulePath, nil
}

// RootModulePath returns the root module path for the current working directory.
// This is a convenience function that calls ModulePath with the current directory.
func RootModulePath(ctx context.Context) (string, error) {
	// Getwd returns an absolute path name corresponding to the current directory.
	wd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("getting working directory: %w", err)
	}
	return modulePath(ctx, wd)
}
