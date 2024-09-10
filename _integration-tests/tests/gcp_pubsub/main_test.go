// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

//go:build integration || gcp_pubsub

package gcp_pubsub

import (
	"testing"

	"github.com/stretchr/testify/require"

	"orchestrion/integration/utils/agent"
	"orchestrion/integration/validator/trace"
)

//dd:orchestrion-enabled
const orchestrionEnabled = false

func Test(t *testing.T) {
	require.True(t, orchestrionEnabled, "this test suite must be run with orchestrion enabled")

	mockAgent, err := agent.New(t)
	require.NoError(t, err)
	defer mockAgent.Close()

	tc := &TestCase{}
	t.Log("Running setup")
	tc.Setup(t)

	defer func() {
		t.Log("Running teardown")
		tc.Teardown(t)
	}()

	sess, err := mockAgent.NewSession(t)
	require.NoError(t, err)

	// Defer this, so it runs even if the test panics (e.g, as the result of a failed assertion).
	// If this does not happen, the test session will remain open; which is undesirable.
	defer checkTrace(t, tc, sess)

	t.Log("Running test")
	tc.Run(t)
}

func checkTrace(t *testing.T, tc *TestCase, sess *agent.Session) {
	t.Helper()

	jsonTraces, err := sess.Close(t)
	require.NoError(t, err)

	var traces trace.Spans
	require.NoError(t, trace.ParseRaw(jsonTraces, &traces))
	t.Logf("Received %d traces", len(traces))

	for _, expected := range tc.ExpectedTraces() {
		expected.RequireAnyMatch(t, traces)
	}
}
