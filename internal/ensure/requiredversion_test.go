// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package ensure

import (
	"errors"
	"os/exec"
	"sync/atomic"
	"testing"

	"github.com/datadog/orchestrion/internal/version"
	"github.com/stretchr/testify/require"
)

func Test(t *testing.T) {
	goBin, err := exec.LookPath("go")
	require.NoError(t, err, "could not resolve go command path")

	testError := errors.New("simulated failure")
	osArgs := []string{"/path/to/go/compile", "-a", "./..."}

	type goModVersionResult struct {
		version string
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
		"go.mod failure": {
			goModVersion: goModVersionResult{err: testError},
			expected:     expectedOutcome{err: testError},
		},
		"respawn needed (requires different version)": {
			goModVersion: goModVersionResult{version: "v1337.42.0-phony.0"},
			expected:     expectedOutcome{respawns: true},
		},
		"respawn needed (blank required version)": {
			goModVersion: goModVersionResult{version: ""},
			expected:     expectedOutcome{respawns: true},
		},
		"respawn loop": {
			goModVersion:    goModVersionResult{version: "v1337.42.0-phony.0"},
			envVarRespawned: "v1.2.3-example.1",
			expected:        expectedOutcome{err: errRespawnLoop},
		},
	} {
		t.Run(name, func(t *testing.T) {
			mockGoVersion := func() (string, error) {
				return tc.goModVersion.version, tc.goModVersion.err
			}
			mockGetenv := func(name string) string {
				require.Equal(t, name, envVarRespawned)
				return tc.envVarRespawned
			}
			var syscallExecCalled atomic.Bool
			mockSyscallExec := func(arg0 string, args []string, env []string) error {
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
