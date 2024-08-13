// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package toolexec

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"

	"github.com/datadog/orchestrion/internal/injector/builtin"
	"github.com/datadog/orchestrion/internal/jobserver/client"
	"github.com/datadog/orchestrion/internal/toolexec/proxy"
	"github.com/otiai10/copy"
	"github.com/stretchr/testify/require"
	"golang.org/x/tools/go/packages"
)

var (
	rootDir        string
	localToolchain bool
	goMinorVersion int
)

func init() {
	_, file, _, _ := runtime.Caller(0)
	rootDir = filepath.Join(file, "..", "..", "..")

	localToolchain = os.Getenv("GOTOOLCHAIN") == "local"

	goMinorVersion, _ = strconv.Atoi(strings.Split(runtime.Version(), ".")[1])
}

func Test(t *testing.T) {
	t.Setenv(client.ENV_VAR_JOBSERVER_URL, "") // Make sure we don't accidentally connect to an external jobserver...

	tmp := t.TempDir()
	runGo(t, tmp, "mod", "init", "github.com/DataDog/phony/package")

	// We "go get" the same versions of the dependencies of this package, so that we don't need them resolved again. This
	// helps avoid issues where `go get` may fail on packages such as `k8s.io/client-go` because it decided to declare
	// `toolchain 1.22.0` starting at v0.33.0, but we need `go1.21`-compatible packages only at this point.
	getArgs := append(make([]string, 0, len(builtin.InjectedPaths)), "get", fmt.Sprintf("go@%s", runtime.Version()[2:]))
	pkgs, err := packages.Load(&packages.Config{Mode: packages.NeedName | packages.NeedModule, Logf: t.Logf}, builtin.InjectedPaths[:]...)
	require.NoError(t, err)
	dedup := make(map[string]struct{}, len(pkgs))
	for _, pkg := range pkgs {
		if pkg.Module == nil {
			// Those are the standard library packages!
			continue
		}

		spec := pkg.Module.Path
		if _, isDup := dedup[spec]; isDup {
			continue
		}
		dedup[spec] = struct{}{}

		if pkg.Module.Version != "" {
			spec = fmt.Sprintf("%s@%s", spec, pkg.Module.Version)
		}

		getArgs = append(getArgs, spec)
	}
	runGo(t, tmp, getArgs...)

	// Add a go source file to make sure the toolchain doesn't complain we need to run `go mod tidy`...
	var depsGo bytes.Buffer
	fmt.Fprintln(&depsGo, "//go:build tools")
	fmt.Fprintln(&depsGo, "package main")
	fmt.Fprintln(&depsGo, "import (")
	for _, pkg := range builtin.InjectedPaths {
		fmt.Fprintf(&depsGo, "\t_ %q\n", pkg)
	}
	fmt.Fprintln(&depsGo, ")")
	require.NoError(t, os.WriteFile(filepath.Join(tmp, "deps.go"), depsGo.Bytes(), 0o644))

	// "Fake" proxy command.
	cmd, err := proxy.ParseCommand([]string{"go", "tool", "compile", "-V=full"})
	require.NoError(t, err)

	// Compute the initial version string...
	initial := inDir(t, tmp, func() string {
		v, err := ComputeVersion(cmd)
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

	// Replace the orchestrion package with the copy we just made...
	runGo(t, tmp, "mod", "edit", "-replace", fmt.Sprintf("github.com/datadog/orchestrion=%s", copyDir))
	runGo(t, tmp, "mod", "tidy") // The hash of the dependency has changed... go list would complain...
	updated := inDir(t, tmp, func() string {
		v, err := ComputeVersion(cmd)
		require.NoError(t, err)
		return v
	})
	require.NotEmpty(t, updated)
	require.NotEqual(t, initial, updated)

	// Modify the beacon
	require.NoError(t, os.WriteFile(beaconFile, []byte("package instrument\nconst BEACON = 1337"), 0o644))
	final := inDir(t, tmp, func() string {
		v, err := ComputeVersion(cmd)
		require.NoError(t, err)
		return v
	})
	require.NotEmpty(t, final)
	require.NotEqual(t, initial, final)
	require.NotEqual(t, updated, final)
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
