// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

// go:build integration

package tests

import (
	"testing"

	"orchestrion/integration/utils/agent"
	"orchestrion/integration/validator/trace"

	"github.com/stretchr/testify/require"
)

var orchestrionEnabled bool // Modified by orchestrion itself

func Test(t *testing.T) {
	require.True(t, orchestrionEnabled, "this test suite must be run with orchestrion enabled")
	require.NotEmpty(t, suite, "no test case registered")

	mockAgent, err := agent.New(t)
	require.NoError(t, err)
	defer mockAgent.Close()

	for name, tc := range suite {
		tc := tc
		t.Run(name, func(t *testing.T) {
			t.Log("Running setup")
			tc.Setup(t)

			defer func() {
				t.Log("Running teardown")
				tc.Teardown(t)
			}()

			sess, err := mockAgent.NewSession(t)
			require.NoError(t, err)
			defer func() {
				jsonTraces, err := sess.Close(t)
				require.NoError(t, err)

				var traces trace.Spans
				require.NoError(t, trace.ParseRaw(jsonTraces, &traces))
				t.Logf("Received %d traces", len(traces))

				for _, expected := range tc.ExpectedTraces() {
					matches, diffs := expected.MatchesAny(traces)
					require.True(t, matches, "%s", diffs)
				}
			}()

			t.Log("Running test")
			tc.Run(t)
		})
	}
}
