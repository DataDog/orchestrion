// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package gomod

import (
	"cmp"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
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

	t.Run("go directive with a trailing comment", func(t *testing.T) {
		// The go.work file is already newer than the module, so it must not be
		// lowered to the module's version.
		moduleDir, modfile := writeModule(t, "1.26.0")
		workfile := filepath.Join(filepath.Dir(moduleDir), "go.work")
		original := "go 1.26.4 // keep in sync with CI\n\nuse ./server\n"
		require.NoError(t, os.WriteFile(workfile, []byte(original), 0o644))

		require.NoError(t, AlignWorkGoVersion(ctx, moduleDir, modfile))

		updated, err := os.ReadFile(workfile)
		require.NoError(t, err)
		assert.Equal(t, original, string(updated), "a newer go.work must not be downgraded")
	})

	t.Run("no workspace", func(t *testing.T) {
		// The module directory is created under a fresh t.TempDir tree, so no
		// go.work file encloses it.
		moduleDir, modfile := writeModule(t, "1.26.0")

		assert.NoError(t, AlignWorkGoVersion(ctx, moduleDir, modfile))
	})
}

func TestRunEditWorkspaceVendor(t *testing.T) {
	// Regression test for https://github.com/DataDog/orchestrion/issues/572: when
	// the module belongs to a workspace vendored with `go work vendor`, RunEdit
	// must re-vendor the workspace, instead of replacing its vendor directory
	// with one for the module alone (go.work next to go.mod), or leaving it out
	// of date (go.work in a parent directory).
	ctx := context.Background()

	goCmd := func(t *testing.T, dir string, args ...string) {
		t.Helper()
		cmd := exec.CommandContext(ctx, "go", args...)
		cmd.Dir = dir
		out, err := cmd.CombinedOutput()
		require.NoError(t, err, string(out))
	}

	for name, tc := range map[string]struct {
		// The directories of go.mod and go.work, relative to the fixture's root.
		moduleRel string
		workRel   string
		// The `go` directive of go.mod and go.work, if below the dependency's.
		goVersion string
		// GOWORK is "off", or names the go.work file, while RunEdit runs.
		goworkOff      bool
		explicitGOWORK bool
		// The fixture is vendored with `GOWORK=off go mod vendor`, or not at all,
		// instead of with `go work vendor`.
		modVendor  bool
		unvendored bool
		// The workspace has a sibling module, with a dependency of its own.
		sibling bool
	}{
		"go.work next to go.mod":        {moduleRel: ".", sibling: true},
		"go.work in a parent directory": {moduleRel: "app"},
		// Workspace mode is disabled: the module is vendored on its own, as before.
		"GOWORK=off": {moduleRel: ".", goworkOff: true},
		// The vendor directory is the module's own: it is vendored on its own, as before.
		"module vendored with go mod vendor next to go.work": {moduleRel: ".", modVendor: true},
		// There is no vendor directory to update.
		"unvendored workspace": {moduleRel: ".", unvendored: true},
		// `go mod tidy` raises the module's `go` directive, so go.work's must follow.
		"dependency requiring a newer go version": {moduleRel: "app", goVersion: "1.25.0"},
		// The go.work file named by GOWORK is not found by walking up from go.mod.
		"GOWORK naming a go.work outside the module's parent directories": {moduleRel: "app", workRel: "ws", goVersion: "1.25.0", explicitGOWORK: true},
	} {
		t.Run(name, func(t *testing.T) {
			goVersion := cmp.Or(tc.goVersion, "1.26.0")

			// A local dependency, so that no network access is needed.
			depDir := t.TempDir()
			require.NoError(t, os.WriteFile(filepath.Join(depDir, "go.mod"), []byte("module example.com/dep\n\ngo 1.26.0\n"), 0o644))
			require.NoError(t, os.WriteFile(filepath.Join(depDir, "dep.go"), []byte("package dep\n"), 0o644))

			root := t.TempDir()
			workDir := filepath.Join(root, tc.workRel)
			moduleDir := filepath.Join(root, tc.moduleRel)
			require.NoError(t, os.MkdirAll(workDir, 0o755))
			require.NoError(t, os.MkdirAll(moduleDir, 0o755))
			modfile := filepath.Join(moduleDir, "go.mod")
			require.NoError(t, os.WriteFile(modfile, []byte("module example.com/app\n\ngo "+goVersion+"\n"), 0o644))

			use, err := filepath.Rel(workDir, moduleDir)
			require.NoError(t, err)
			uses := []string{filepath.ToSlash(use)}
			siblingDir := filepath.Join(root, "sibling")
			if tc.sibling {
				siblingDep := t.TempDir()
				require.NoError(t, os.WriteFile(filepath.Join(siblingDep, "go.mod"), []byte("module example.com/siblingdep\n\ngo 1.26.0\n"), 0o644))
				require.NoError(t, os.WriteFile(filepath.Join(siblingDep, "dep.go"), []byte("package siblingdep\n"), 0o644))
				require.NoError(t, os.MkdirAll(siblingDir, 0o755))
				require.NoError(t, os.WriteFile(filepath.Join(siblingDir, "go.mod"), []byte("module example.com/sibling\n\ngo 1.26.0\n\nrequire example.com/siblingdep v0.0.0\n\nreplace example.com/siblingdep => "+siblingDep+"\n"), 0o644))
				require.NoError(t, os.WriteFile(filepath.Join(siblingDir, "sibling.go"), []byte("package sibling\n\nimport _ \"example.com/siblingdep\"\n"), 0o644))
				uses = append(uses, "./sibling")
			}
			workfile := filepath.Join(workDir, "go.work")
			require.NoError(t, os.WriteFile(workfile, []byte("go "+goVersion+"\n\nuse (\n\t"+strings.Join(uses, "\n\t")+"\n)\n"), 0o644))

			t.Setenv("GOWORK", "auto")
			switch {
			case tc.modVendor:
				// `go mod vendor` creates no vendor directory for a module without requirements.
				t.Setenv("GOWORK", "off")
				goCmd(t, moduleDir, "mod", "edit", "-require=example.com/dep@v0.0.0", "-replace=example.com/dep="+depDir)
				goCmd(t, moduleDir, "mod", "vendor")
				t.Setenv("GOWORK", "auto")
			case !tc.unvendored:
				goCmd(t, workDir, "work", "vendor")
			}

			// The module starts using a new dependency, as `orchestrion pin` does.
			require.NoError(t, os.WriteFile(filepath.Join(moduleDir, "main.go"), []byte("package main\n\nimport _ \"example.com/dep\"\n\nfunc main() {}\n"), 0o644))
			t.Chdir(moduleDir)
			if tc.goworkOff {
				t.Setenv("GOWORK", "off")
			}
			if tc.explicitGOWORK {
				t.Setenv("GOWORK", workfile)
			}
			require.NoError(t, RunEdit(ctx, modfile,
				Require{Path: "example.com/dep", Version: "v0.0.0"},
				Replace{OldPath: "example.com/dep", NewPath: depDir},
			))

			if tc.unvendored {
				assert.NoDirExists(t, filepath.Join(workDir, "vendor"))
				return
			}
			workspaceVendor := !tc.goworkOff && !tc.modVendor
			modulesTxt, err := os.ReadFile(filepath.Join(workDir, "vendor", "modules.txt"))
			require.NoError(t, err)
			assert.Equal(t, workspaceVendor, strings.HasPrefix(string(modulesTxt), "## workspace\n"), "vendor/modules.txt:\n%s", modulesTxt)
			if workspaceVendor {
				// `go mod tidy` raised the module's `go` directive to the dependency's.
				work, err := os.ReadFile(workfile)
				require.NoError(t, err)
				assert.Contains(t, string(work), "go 1.26.0\n")
			} else {
				// Only builds outside of workspace mode use the module's own vendor directory.
				t.Setenv("GOWORK", "off")
			}
			goCmd(t, moduleDir, "list", "-mod=vendor", "./...")
			if tc.sibling {
				goCmd(t, siblingDir, "list", "-mod=vendor", "./...")
			}
		})
	}

	t.Run("unreadable vendor/modules.txt", func(t *testing.T) {
		workDir := t.TempDir()
		modfile := filepath.Join(workDir, "go.mod")
		require.NoError(t, os.WriteFile(modfile, []byte("module example.com/app\n\ngo 1.26.0\n"), 0o644))
		require.NoError(t, os.WriteFile(filepath.Join(workDir, "go.work"), []byte("go 1.26.0\n\nuse .\n"), 0o644))
		// Reading a directory fails with an error other than fs.ErrNotExist.
		require.NoError(t, os.MkdirAll(filepath.Join(workDir, "vendor", "modules.txt"), 0o755))
		t.Setenv("GOWORK", "auto")

		_, err := VendorWorkspace(ctx, modfile)
		assert.ErrorContains(t, err, "modules.txt")
	})
}
