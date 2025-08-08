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
	// Cache for module path lookups to avoid repeated calls to go list.
	modulePathCache = make(map[string]string)
	modulePathMutex sync.RWMutex
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

// ModulePath returns the module path of the current module (from running `go list -m`).
// Results are cached to avoid repeated calls to go list.
func ModulePath(ctx context.Context, dir string) (string, error) {
	modulePathMutex.RLock()
	if cached, exists := modulePathCache[dir]; exists {
		modulePathMutex.RUnlock()
		return cached, nil
	}
	modulePathMutex.RUnlock()

	cmd := exec.CommandContext(ctx, "go", "list", "-m")
	cmd.Dir = dir
	var stdout bytes.Buffer
	cmd.Stdout = &stdout
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("running %q: %w", cmd.Args, err)
	}

	modulePath := strings.TrimSpace(stdout.String())
	if modulePath == "" {
		return "", fmt.Errorf("in %q: %w", dir, ErrNoModulePath)
	}

	modulePathMutex.Lock()
	modulePathCache[dir] = modulePath
	modulePathMutex.Unlock()

	return modulePath, nil
}

// RootModulePath returns the root module path for the current working directory.
// This is a convenience function that calls ModulePath with the current directory.
func RootModulePath(ctx context.Context) (string, error) {
	wd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("getting working directory: %w", err)
	}
	return ModulePath(ctx, wd)
}
