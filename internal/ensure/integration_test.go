// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package ensure_test

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/DataDog/orchestrion/internal/version"
	"github.com/stretchr/testify/require"
	"golang.org/x/mod/semver"
)

var ensureDir string

func Test(t *testing.T) {
	tmp, err := os.MkdirTemp("", "ensure_test-*")
	require.NoError(t, err, "failed to create temporary working directory")
	defer os.RemoveAll(tmp)

	testMain := filepath.Join(tmp, "bin", "test_main")

	_, err = shell(ensureDir, "go", "build", "-o", testMain, "./integration")
	require.NoError(t, err, "failed to build test_main helper")

	type test struct {
		// version is the orchestrion version to mention in the `go.mod` files' require directive. The
		// test will run go mod tidy, so this version must be an existing release tag. We typically use
		// `v0.6.0` for testing purposes, but you can specify another existing version if there is a
		// reason to. If blank, no `require` directive will be added in `go.mod` to require orchestrion.
		version string
		// replaces causes the `go.mod` file to have a `replace` directive that redirects the
		// Orchestrion package to the version that is currently being tested.
		replaces bool
		// output is the expected output from running the `test_main` command, which is created from
		// compiling the `./integration` package.
		output string
		// fails is true when the `test_main` helper is expected to exit with a non-0 status code. When
		// true, the value of `output` is not asserted against.
		fails bool
	}
	for name, test := range map[string]test{
		"v0.9.0":   {version: "v0.9.0", output: "orchestrion v0.9.0"},
		"replaced": {version: "v0.9.0", replaces: true, output: "This command has not respawned!"},
		"none":     {fails: true},
	} {
		t.Run(name, func(t *testing.T) {
			if !test.replaces && semver.Compare(test.version, version.Tag) >= 0 {
				// Tests w/o replace can't run if the "happy" version has not been released yet. v0.9.0 includes a module path
				// re-capitalization which forces us to skip temporarily at least until that is released.
				t.Skipf("Skipping test because version %s is newer than the current version (%s)", test.version, version.Tag)
			}

			wd := filepath.Join(tmp, name)
			require.NoError(t, os.Mkdir(wd, 0750), "failed to create test working directory")

			goMod := []string{
				"module integration_test_case",
				"",
				fmt.Sprintf("go %s", runtime.Version()[2:]),
				"",
			}

			if test.version != "" {
				goMod = append(goMod, fmt.Sprintf("require github.com/DataDog/orchestrion %s", test.version), "")

				// So that "go mod tidy" does not remove the requirement...
				require.NoError(t,
					os.WriteFile(filepath.Join(wd, "tools.go"), []byte(strings.Join([]string{
						"//go:build tools",
						"package tools",
						"",
						"import _ \"github.com/DataDog/orchestrion\"",
					}, "\n")), 0o644),
					"failed to write tools.go",
				)
			}
			if test.replaces {
				goMod = append(goMod, fmt.Sprintf("replace github.com/DataDog/orchestrion => %s", filepath.Dir(filepath.Dir(ensureDir))), "")
			}

			require.NoError(t,
				os.WriteFile(filepath.Join(wd, "go.mod"), []byte(strings.Join(goMod, "\n")), 0o644),
				"failed to create go.mod file",
			)

			_, err := shell(wd, "go", "mod", "tidy")
			require.NoError(t, err, "failed to 'go mod tidy'")

			out, err := shell(wd, testMain, "version")
			if test.fails {
				_, ok := err.(*exec.ExitError)
				require.True(t, ok, "unexpected error while running test_main: %v", err)
			} else {
				require.NoError(t, err, "failed to run test_main helper")
				require.Equal(t, test.output, out, "unexpected output from test_main helper")
			}
		})
	}
}

func shell(dir string, cmd string, args ...string) (string, error) {
	var stdout bytes.Buffer

	child := exec.Command(cmd, args...)
	child.Dir = dir
	child.Stdout = &stdout

	err := child.Run()
	return strings.TrimSpace(stdout.String()), err
}

func init() {
	_, file, _, _ := runtime.Caller(0)
	ensureDir = filepath.Dir(file)
}
