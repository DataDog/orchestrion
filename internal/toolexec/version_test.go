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
	"strings"
	"testing"

	"github.com/datadog/orchestrion/internal/injector/builtin"
	"github.com/datadog/orchestrion/internal/toolexec/proxy"
	"github.com/otiai10/copy"
	"github.com/stretchr/testify/require"
)

var rootDir string

func init() {
	_, file, _, _ := runtime.Caller(0)
	rootDir = filepath.Join(file, "..", "..", "..")
}

func Test(t *testing.T) {
	tmp := t.TempDir()
	runGo(t, tmp, "mod", "init", "github.com/DataDog/phony/package")

	getArgs := []string{"get"}
	for _, pkg := range builtin.InjectedPaths {
		// We don't want to try to "go get" standard library packages; these don't contain a ".".
		if strings.Contains(pkg, ".") {
			getArgs = append(getArgs, pkg)
		}
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
	initial := inDir(t, tmp, func() string { return ComputeVersion(cmd, "/dev/null") })
	require.NotEmpty(t, initial)

	copyDir := t.TempDir()
	require.NoError(t, copy.Copy(rootDir, copyDir, copy.Options{
		Skip: func(src string) (bool, error) {
			return filepath.Base(src) == ".git", nil
		},
	}))
	beaconFile := filepath.Join(copyDir, "instrument", "beacon___.go")
	require.NoError(t, os.WriteFile(beaconFile, []byte("package instrument\nconst BEACON = 42"), 0o644))

	// Replace the orchestrion package with the copy we just made...
	runGo(t, tmp, "mod", "edit", "-replace", fmt.Sprintf("github.com/datadog/orchestrion=%s", copyDir))
	updated := inDir(t, tmp, func() string { return ComputeVersion(cmd, "/dev/null") })
	require.NotEmpty(t, updated)
	require.NotEqual(t, initial, updated)

	// Modify the beacon
	require.NoError(t, os.WriteFile(beaconFile, []byte("package instrument\nconst BEACON = 1337"), 0o644))
	final := inDir(t, tmp, func() string { return ComputeVersion(cmd, "/dev/null") })
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
