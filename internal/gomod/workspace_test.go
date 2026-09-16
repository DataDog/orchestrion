// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package gomod

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAlignWorkGoVersion(t *testing.T) {
	ctx := context.Background()

	// writeModule creates a module in a fresh temporary directory, and returns
	// its directory and go.mod file path.
	writeModule := func(t *testing.T, goVersion string) (moduleDir string, modfile string) {
		t.Helper()
		dir := t.TempDir()
		moduleDir = filepath.Join(dir, "server")
		require.NoError(t, os.MkdirAll(moduleDir, 0o755))
		modfile = filepath.Join(moduleDir, "go.mod")
		require.NoError(t, os.WriteFile(modfile, []byte("module example.com/server\n\ngo "+goVersion+"\n"), 0o644))
		return moduleDir, modfile
	}

	t.Run("raises a stale go.work go directive", func(t *testing.T) {
		// etcd-style monorepo: the go.work file uses the two-part form, which
		// does not satisfy the module's three-part requirement. This is the
		// state `go mod tidy` leaves behind when it raises the module's `go`
		// directive (e.g. after pinning a dependency needing go 1.26.0).
		moduleDir, modfile := writeModule(t, "1.26.0")
		workfile := filepath.Join(filepath.Dir(moduleDir), "go.work")
		require.NoError(t, os.WriteFile(workfile, []byte("go 1.26\n\ntoolchain go1.26.5\n\nuse ./server\n"), 0o644))

		require.NoError(t, AlignWorkGoVersion(ctx, moduleDir, modfile))

		updated, err := os.ReadFile(workfile)
		require.NoError(t, err)
		assert.Contains(t, string(updated), "go 1.26.0")
		assert.Contains(t, string(updated), "toolchain go1.26.5", "the toolchain directive must be preserved")
	})

	t.Run("leaves an up-to-date go.work untouched", func(t *testing.T) {
		moduleDir, modfile := writeModule(t, "1.26.0")
		workfile := filepath.Join(filepath.Dir(moduleDir), "go.work")
		original := "go 1.26.0\n\nuse (\n\t./server\n)\n"
		require.NoError(t, os.WriteFile(workfile, []byte(original), 0o644))

		require.NoError(t, AlignWorkGoVersion(ctx, moduleDir, modfile))

		updated, err := os.ReadFile(workfile)
		require.NoError(t, err)
		assert.Equal(t, original, string(updated), "an aligned go.work must not be rewritten")
	})

	t.Run("no workspace", func(t *testing.T) {
		// The module directory is created under a fresh t.TempDir tree, so no
		// go.work file encloses it.
		moduleDir, modfile := writeModule(t, "1.26.0")

		assert.NoError(t, AlignWorkGoVersion(ctx, moduleDir, modfile))
	})
}
