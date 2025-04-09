// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package toolexec

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/DataDog/orchestrion/internal/goflags"
	"github.com/DataDog/orchestrion/internal/injector/config"
	"github.com/DataDog/orchestrion/internal/jobserver/client"
	"github.com/DataDog/orchestrion/internal/toolexec/proxy"
	"github.com/otiai10/copy"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"
)

var rootDir string

func init() {
	_, file, _, _ := runtime.Caller(0)
	rootDir = filepath.Join(file, "..", "..", "..")
}

func Test(t *testing.T) {
	t.Setenv(client.EnvVarJobserverURL, "") // Make sure we don't accidentally connect to an external jobserver...

	t.Run("simple", func(t *testing.T) {
		tmp := t.TempDir()
		runGo(t, tmp, "mod", "init", "github.com/DataDog/phony/package")
		runGo(t, tmp, "mod", "edit",
			"-replace=github.com/DataDog/orchestrion="+rootDir,
			"-replace=github.com/DataDog/orchestrion/instrument="+filepath.Join(rootDir, "instrument"),
		)

		require.NoError(t, os.WriteFile(filepath.Join(tmp, config.FilenameOrchestrionToolGo), []byte(`
		//go:build tools
		package tools

		import (
			_ "github.com/DataDog/orchestrion"
			_ "github.com/DataDog/orchestrion/instrument"
		)
	`), 0o644))
		runGo(t, tmp, "mod", "tidy")

		// "Fake" proxy command.
		cmd, err := proxy.ParseCommand(context.Background(), "github.com/DataDog/phony/package", []string{"go", "tool", "compile", "-V=full"})
		require.NoError(t, err)

		ctx := zerolog.New(zerolog.MultiLevelWriter(zerolog.NewTestWriter(t))).
			Level(zerolog.InfoLevel).
			WithContext(context.Background())

		// Artificially set goflags as this is needed for the test to run....
		goflags.SetFlags(ctx, tmp, []string{"test", "./..."})

		// Compute the initial version string...
		initial := inDir(t, tmp, func() string {
			v, err := ComputeVersion(ctx, cmd)
			require.NoError(t, err)
			return v
		})
		require.NotEmpty(t, initial)

		copyDir := t.TempDir()
		require.NoError(t, copy.Copy(rootDir, copyDir, copy.Options{
			Skip: func(_ os.FileInfo, src string, _ string) (bool, error) {
				return filepath.Base(src) == ".git", nil
			},
		}))
		beaconFile := filepath.Join(copyDir, "instrument", "beacon___.go")
		require.NoError(t, os.WriteFile(beaconFile, []byte("package instrument\nconst BEACON = 42"), 0o644))
		// Add an aspect that looks like it may inject github.com/DataDog/orchestrion/instrument
		require.NoError(t, os.WriteFile(
			filepath.Join(copyDir, "instrument", config.FilenameOrchestrionYML),
			[]byte(`aspects: [{join-point: {test-main: true}, advice: [{inject-declarations: {imports: {i: 'github.com/DataDog/orchestrion/instrument'}}}]}]`),
			0o644,
		))

		// Replace the orchestrion package with the copy we just made...
		runGo(t, tmp, "mod", "edit",
			"-replace=github.com/DataDog/orchestrion="+copyDir,
			"-replace=github.com/DataDog/orchestrion/instrument="+filepath.Join(copyDir, "instrument"),
		)
		runGo(t, tmp, "mod", "tidy") // The hash of the dependency has changed... go list would complain...
		updated := inDir(t, tmp, func() string {
			v, err := ComputeVersion(ctx, cmd)
			require.NoError(t, err)
			return v
		})
		require.NotEmpty(t, updated)
		require.NotEqual(t, initial, updated)

		// Modify the beacon
		require.NoError(t, os.WriteFile(beaconFile, []byte("package instrument\nconst BEACON = 1337"), 0o644))
		final := inDir(t, tmp, func() string {
			v, err := ComputeVersion(ctx, cmd)
			require.NoError(t, err)
			return v
		})
		require.NotEmpty(t, final)
		require.NotEqual(t, initial, final)
		require.NotEqual(t, updated, final)
	})

	t.Run("workspace", func(t *testing.T) {
		tmp := t.TempDir()

		// Initialize the workspace...
		runGo(t, tmp, "work", "init")
		runGo(t, tmp, "work", "edit",
			"-replace=github.com/DataDog/orchestrion="+rootDir,
			"-replace=github.com/DataDog/orchestrion/instrument="+filepath.Join(rootDir, "instrument"),
		)

		// Create the cmd/main package...
		pkgMain := filepath.Join(tmp, "cmd", "main")
		require.NoError(t, os.MkdirAll(pkgMain, 0o755))
		runGo(t, pkgMain, "mod", "init", "github.com/DataDog/phony/cmd/main")
		runGo(t, tmp, "work", "use", filepath.Join("cmd", "main"))
		require.NoError(t, os.WriteFile(filepath.Join(pkgMain, config.FilenameOrchestrionToolGo), []byte(`
		//go:build tools
		package tools

		import (
			_ "github.com/DataDog/orchestrion"
			_ "github.com/DataDog/phony/pkg/dep"
		)
		`), 0o644))
		require.NoError(t, os.WriteFile(filepath.Join(pkgMain, config.FilenameOrchestrionYML),
			[]byte(`aspects: [{join-point: {test-main: true}, advice: [{inject-declarations: {imports: {i: 'github.com/DataDog/phony/pkg/dep'}}}]}]`),
			0o644,
		))

		// Create the pkg/dep package...
		pkgDep := filepath.Join(tmp, "pkg", "dep")
		require.NoError(t, os.MkdirAll(pkgDep, 0o755))
		runGo(t, pkgDep, "mod", "init", "github.com/DataDog/phony/pkg/dep")
		runGo(t, tmp, "work", "use", filepath.Join("pkg", "dep"))
		require.NoError(t, os.WriteFile(filepath.Join(pkgDep, "dep.go"), []byte(`package dep
		const BEACON = 1337
		`), 0o644))
		runGo(t, pkgMain, "mod", "edit", "-replace=github.com/DataDog/phony/pkg/dep=../../pkg/dep")

		// Now let's get to business... Initialize a new context with a logger (for troubleshooting failures):
		ctx := zerolog.New(zerolog.MultiLevelWriter(zerolog.NewTestWriter(t))).
			Level(zerolog.InfoLevel).
			WithContext(context.Background())
		// "Fake" proxy command (input to ComputeVersion).
		cmd, err := proxy.ParseCommand(context.Background(), "github.com/DataDog/phony/cmd/main", []string{"go", "tool", "compile", "-V=full"})
		require.NoError(t, err)

		// Artificially set goflags as this is needed for the test to run....
		goflags.SetFlags(ctx, pkgMain, []string{"test", "./..."})

		// Compute the initial version string...
		initial := inDir(t, pkgMain, func() string {
			v, err := ComputeVersion(ctx, cmd)
			require.NoError(t, err)
			return v
		})
		require.NotEmpty(t, initial)

		// Now modify the injected dependency...
		require.NoError(t, os.WriteFile(filepath.Join(pkgDep, "dep.go"), []byte(`package dep
		const BEACON = 42
		`), 0o644))

		// Compute the updated version string...
		updated := inDir(t, pkgMain, func() string {
			v, err := ComputeVersion(ctx, cmd)
			require.NoError(t, err)
			return v
		})
		require.NotEmpty(t, updated)

		require.NotEqual(t, initial, updated)
	})
}

func inDir[T any](t *testing.T, wd string, cb func() T) T {
	orig, err := os.Getwd()
	require.NoError(t, err)
	defer func() { require.NoError(t, os.Chdir(orig)) }()

	require.NoError(t, os.Chdir(wd))
	return cb()
}

func runGo(t *testing.T, wd string, args ...string) {
	cmd := exec.Command("go", args...)
	cmd.Dir = wd
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	require.NoError(t, cmd.Run(), "failed to run 'go %s'", strings.Join(args, " "))
}
