// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package ensure

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/DataDog/orchestrion/internal/goenv"
	"github.com/DataDog/orchestrion/internal/version"
	"github.com/stretchr/testify/require"
	"golang.org/x/mod/semver"
)

func TestGoModVersion(t *testing.T) {
	type test struct {
		version string
		replace bool
		err     error
	}
	for name, test := range map[string]test{
		"happy":    {version: "v0.9.0"},
		"replaced": {version: "v0.9.0", replace: true},
		"missing":  {err: fmt.Errorf("no required module provides package %s", orchestrionPkgPath)},
	} {
		t.Run(name, func(t *testing.T) {
			if !test.replace && test.version != "" && semver.Compare(test.version, version.Tag) >= 0 {
				// Tests w/o replace can't run if the "happy" version has not been released yet. v0.9.0 includes a module path
				// re-capitalization which forces us to skip temporarily at least until that is released.
				t.Skipf("Skipping test because version %s is newer than the current version (%s)", test.version, version.Tag)
			}

			tmp, err := os.MkdirTemp("", "ensure-*")
			require.NoError(t, err, "failed to create temporary directory")
			defer os.RemoveAll(tmp)

			goMod := []string{
				"module test_case",
				"",
				fmt.Sprintf("go %s", runtime.Version()[2:]),
				"",
			}
			if test.version != "" {
				goMod = append(goMod, fmt.Sprintf("require %s %s", orchestrionPkgPath, test.version), "")
				require.NoError(t,
					os.WriteFile(filepath.Join(tmp, "tools.go"), []byte(fmt.Sprintf("//go:build tools\npackage tools\n\nimport _ %q\n", orchestrionPkgPath)), 0o644),
					"failed to write tools.go",
				)
			}
			if test.replace {
				goMod = append(goMod, fmt.Sprintf("replace %s => %s", orchestrionPkgPath, orchestrionSrcDir), "")
			}

			require.NoError(t, os.WriteFile(filepath.Join(tmp, "go.mod"), []byte(strings.Join(goMod, "\n")), 0o644), "failed to write go.mod file")

			child := exec.Command("go", "mod", "tidy")
			child.Dir = tmp
			child.Stderr = os.Stderr
			require.NoError(t, child.Run(), "error while running 'go mod tidy'")

			rVersion, rDir, err := goModVersion(tmp)
			if test.err != nil {
				require.ErrorContains(t, err, test.err.Error())
				return
			}

			require.NoError(t, err)
			if test.replace {
				require.Equal(t, "", rVersion)
				require.Equal(t, orchestrionSrcDir, rDir)
			} else {
				require.Equal(t, test.version, rVersion)
				// In this case, the source tree will be in the GOMODCACHE directory.
				require.Contains(t, rDir, os.Getenv("GOMODCACHE"))
			}
		})
	}

	t.Run("no-go-mod", func(t *testing.T) {
		tmp, err := os.MkdirTemp("", "ensure-*")
		require.NoError(t, err, "failed to create temporary directory")
		defer os.RemoveAll(tmp)

		os.WriteFile(filepath.Join(tmp, "main.go"), []byte(`
package main

func main() {}
		`), 0o644)

		require.NotPanics(t, func() {
			_, _, err = goModVersion(tmp)
		})
		require.ErrorContains(t, err, "no module information found for package")
	})
}

func TestRequiredVersion(t *testing.T) {
	goBin, err := goenv.GoBinPath()
	require.NoError(t, err, "could not resolve go command path")

	testError := errors.New("simulated failure")
	osArgs := []string{"/path/to/go/compile", "-a", "./..."}

	type goModVersionResult struct {
		version string
		path    string
		err     error
	}
	type expectedOutcome struct {
		err      error
		respawns bool
	}
	type testCase struct {
		goModVersion    goModVersionResult
		envVarRespawned string
		expected        expectedOutcome
	}

	for name, tc := range map[string]testCase{
		"happy path": {
			goModVersion: goModVersionResult{version: version.Tag},
			expected:     expectedOutcome{err: nil, respawns: false},
		},
		"happy path, replaced to this": {
			goModVersion: goModVersionResult{path: orchestrionSrcDir},
			expected:     expectedOutcome{err: nil, respawns: false},
		},
		"go.mod failure": {
			goModVersion: goModVersionResult{err: testError},
			expected:     expectedOutcome{err: testError},
		},
		"respawn needed (requires different version)": {
			goModVersion: goModVersionResult{version: "v1337.42.0-phony.0"},
			expected:     expectedOutcome{respawns: true},
		},
		"respawn needed (blank required version, blank path)": {
			goModVersion: goModVersionResult{},
			expected:     expectedOutcome{respawns: true},
		},
		"respawn needed (blank required version, mismatched path)": {
			goModVersion: goModVersionResult{path: "/phony/orchestrion/path"},
			expected:     expectedOutcome{respawns: true},
		},
		"respawn loop": {
			goModVersion:    goModVersionResult{version: "v1337.42.0-phony.0"},
			envVarRespawned: "v1.2.3-example.1",
			expected:        expectedOutcome{err: errRespawnLoop},
		},
	} {
		t.Run(name, func(t *testing.T) {
			mockGoVersion := func(dir string) (string, string, error) {
				require.Equal(t, "", dir)
				return tc.goModVersion.version, tc.goModVersion.path, tc.goModVersion.err
			}
			mockGetenv := func(name string) string {
				require.Equal(t, envVarRespawnedFor, name)
				return tc.envVarRespawned
			}
			var syscallExecCalled atomic.Bool
			mockSyscallExec := func(arg0 string, args []string, _ []string) error {
				t.Helper()
				syscallExecCalled.Store(true)

				require.Equal(t, goBin, arg0)
				require.GreaterOrEqual(t, len(args), 3)
				require.Equal(t, []string{goBin, "run", orchestrionPkgPath}, args[:3])
				require.Equal(t, osArgs[1:], args[3:])

				return nil
			}

			err := requiredVersion(mockGoVersion, mockGetenv, mockSyscallExec, osArgs)

			if tc.expected.err != nil {
				require.ErrorIs(t, err, tc.expected.err)
			} else {
				require.NoError(t, err)
			}
			require.Equal(t, tc.expected.respawns, syscallExecCalled.Load())
		})
	}
}
