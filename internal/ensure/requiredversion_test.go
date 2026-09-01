// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package ensure

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
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
		// The explicit `-mod=readonly` build flag in goModVersion (added to opt
		// out of Go's vendor auto-detection) changes how `go list` phrases a
		// missing-module error compared to its implicit default.
		"missing": {err: fmt.Errorf("cannot find module providing package %s: import lookup disabled by -mod=readonly", orchestrionPkgPath)},
	} {
		t.Run(name, func(t *testing.T) {
			if !test.replace && test.version != "" && semver.Compare(test.version, version.Tag()) >= 0 {
				// Tests w/o replace can't run if the "happy" version has not been released yet. v0.9.0 includes a module path
				// re-capitalization which forces us to skip temporarily at least until that is released.
				t.Skipf("Skipping test because version %s is newer than the current version (%s)", test.version, version.Tag())
			}

			tmp, err := os.MkdirTemp("", "ensure-*")
			require.NoError(t, err, "failed to create temporary directory")
			defer os.RemoveAll(tmp)

			goMod := []string{
				"module test_case",
				"",
				"go " + runtime.Version()[2:],
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

			rVersion, rDir, err := goModVersion(context.Background(), tmp)
			if test.err != nil {
				require.ErrorContains(t, err, test.err.Error())
				return
			}

			require.NoError(t, err)
			if test.replace {
				require.Empty(t, rVersion)
				require.Equal(t, orchestrionSrcDir, rDir)
			} else {
				require.Equal(t, test.version, rVersion)
				// In this case, the source tree will be in the GOMODCACHE directory.
				require.Contains(t, rDir, os.Getenv("GOMODCACHE"))
			}
		})
	}

	t.Run("no-go-mod", func(t *testing.T) {
		tmp := t.TempDir()

		os.WriteFile(filepath.Join(tmp, "main.go"), []byte(`
		package main

		func main() {}
				`), 0o644)

		require.NotPanics(t, func() {
			_, _, err := goModVersion(context.Background(), tmp)
			require.ErrorIs(t, err, goenv.ErrNoGoMod)
		})
	})

	t.Run("vendor-inconsistent", func(t *testing.T) {
		// Regression test: goModVersion must not fail with "inconsistent
		// vendoring" merely because some *other* dependency's `go.mod`
		// requirement drifted out of sync with `vendor/modules.txt` (e.g.
		// because something upstream ran a raw `go get` without re-vendoring).
		// This mirrors the scenario fixed for pruneImports in internal/pin/pin.go
		// (see BuildFlags there), except goModVersion must stay strictly
		// read-only: it must not silently add/mutate requirements either, since
		// it runs on every -toolexec build, before any pinning decision is made.
		tmp := t.TempDir()

		// A purely local, replaced dependency: no network access is needed to
		// resolve it, and its "version" is nominal since resolution goes
		// through the `replace` directive regardless.
		fakeModDir := filepath.Join(tmp, "fakemod")
		require.NoError(t, os.MkdirAll(fakeModDir, 0o755))
		require.NoError(t,
			os.WriteFile(filepath.Join(fakeModDir, "go.mod"), []byte("module example.com/fakemod\n\ngo "+runtime.Version()[2:]+"\n"), 0o644),
			"failed to write fakemod go.mod",
		)
		require.NoError(t,
			os.WriteFile(filepath.Join(fakeModDir, "fake.go"), []byte("package fakemod\n\nfunc Hello() string { return \"hello\" }\n"), 0o644),
			"failed to write fakemod source",
		)

		goMod := strings.Join([]string{
			"module test_case",
			"",
			"go " + runtime.Version()[2:],
			"",
			"require (",
			"\t" + orchestrionPkgPath + " v0.9.0",
			"\texample.com/fakemod v0.0.1",
			")",
			"",
			"replace (",
			"\t" + orchestrionPkgPath + " v0.9.0 => " + orchestrionSrcDir,
			"\texample.com/fakemod => " + fakeModDir,
			")",
			"",
		}, "\n")
		require.NoError(t, os.WriteFile(filepath.Join(tmp, "go.mod"), []byte(goMod), 0o644), "failed to write go.mod file")

		// tools.go pulls in orchestrion (mirroring the other sub-tests above);
		// main.go pulls in the fake module so it is a normal (non-`tools`)
		// dependency that `go mod vendor` will actually vendor.
		require.NoError(t,
			os.WriteFile(filepath.Join(tmp, "tools.go"), []byte(fmt.Sprintf("//go:build tools\npackage tools\n\nimport _ %q\n", orchestrionPkgPath)), 0o644),
			"failed to write tools.go",
		)
		require.NoError(t,
			os.WriteFile(filepath.Join(tmp, "main.go"), []byte("package main\n\nimport \"example.com/fakemod\"\n\nfunc main() { fakemod.Hello() }\n"), 0o644),
			"failed to write main.go",
		)

		runIn := func(name string, args ...string) {
			t.Helper()
			cmd := exec.Command(name, args...)
			cmd.Dir = tmp
			out, err := cmd.CombinedOutput()
			require.NoError(t, err, "%s %s: %s", name, strings.Join(args, " "), out)
		}

		runIn("go", "mod", "tidy")
		runIn("go", "mod", "vendor")

		// Simulate a dependency's requirement drifting away from what was
		// vendored (e.g. via a raw `go get` elsewhere) without `vendor/` being
		// re-synced. This is unrelated to orchestrion itself, but is enough to
		// make Go's vendor-consistency check fail for *any* `go list` call in
		// this module unless vendor auto-detection is explicitly disabled.
		runIn("go", "mod", "edit", "-require=example.com/fakemod@v0.0.2")

		// Sanity-check that this setup does reproduce the "inconsistent
		// vendoring" failure Go would normally auto-select `-mod=vendor` into,
		// so this test would actually catch a regression if goModVersion ever
		// stopped forcing `-mod=readonly`.
		cmd := exec.Command("go", "list", "./...")
		cmd.Dir = tmp
		out, err := cmd.CombinedOutput()
		require.Error(t, err, "expected inconsistent vendoring to be reproduced, got: %s", out)
		require.Contains(t, string(out), "inconsistent vendoring")

		rVersion, rDir, err := goModVersion(context.Background(), tmp)
		require.NoError(t, err)
		require.Empty(t, rVersion)
		require.Equal(t, orchestrionSrcDir, rDir)
	})
}

func TestRequiredVersion(t *testing.T) {
	testError := errors.New("simulated failure")

	type goModVersionResult struct {
		version string
		path    string
		err     error
	}
	type expectedOutcome = error
	type testCase struct {
		goModVersion goModVersionResult
		expected     expectedOutcome
	}

	rawTag, _ := version.TagInfo()
	for name, tc := range map[string]testCase{
		"happy path": {
			goModVersion: goModVersionResult{version: rawTag},
			expected:     nil,
		},
		"happy path, replaced to this": {
			goModVersion: goModVersionResult{path: orchestrionSrcDir},
			expected:     nil,
		},
		"go.mod failure": {
			goModVersion: goModVersionResult{err: testError},
			expected:     testError,
		},
		"different version required": {
			goModVersion: goModVersionResult{version: "v1337.42.0-phony.0"},
			expected:     IncorrectVersionError{RequiredVersion: "v1337.42.0-phony.0"},
		},
		"blank version and directory": { // This might never happen in the wild
			goModVersion: goModVersionResult{},
			expected:     IncorrectVersionError{RequiredVersion: ""},
		},
		"replaced to a different path": {
			goModVersion: goModVersionResult{path: "/phony/orchestrion/path"},
			expected:     IncorrectVersionError{RequiredVersion: ""},
		},
	} {
		t.Run(name, func(t *testing.T) {
			mockGoVersion := func(_ context.Context, dir string) (string, string, error) {
				require.Empty(t, dir)
				return tc.goModVersion.version, tc.goModVersion.path, tc.goModVersion.err
			}

			err := requiredVersion(context.Background(), mockGoVersion)

			require.ErrorIs(t, err, tc.expected)
		})
	}
}
