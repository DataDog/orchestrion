// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package utils

import (
	"testing"

	"datadoghq.dev/orchestrion/_integration-tests/utils/agent"
	"datadoghq.dev/orchestrion/_integration-tests/validator/trace"
	"github.com/DataDog/orchestrion/runtime/built"
	"github.com/stretchr/testify/require"
)

// TestCase describes the general contract for tests. Each package in this
// directory is expected to export a [TestCase] structure implementing this
// interface.
type TestCase interface {
	// Setup is called before the test is run. It should be used to prepare any
	// the test for execution, such as starting up services (e.g, databse servers)
	// or setting up test data. The Setup function can call [testing.T.SkipNow] to
	// skip the test entirely, for example if prerequisites of its dependencies
	// are not satisfied by the test environment.
	//
	// The tracer is not yet started when Setup is executed.
	Setup(*testing.T)

	// Run executes the test case after starting the tracer. This should perform
	// the necessary calls to produce trace information from injected
	// instrumentation, and assert on expected post-conditions (e.g, HTTP request
	// is expected to be successful, database call does not error out, etc...).
	// The tracer is shut down after the Run function returns, ensuring
	// outstanding spans are flushed to the agent.
	Run(*testing.T)

	// ExpectedTraces returns a trace.Traces object describing all traces expected
	// to be produced by the [TestCase.Run] function. There should be one entry
	// per trace root span expected to be produced. Every item in the returned
	// [trace.Traces] must match at least one trace received by the agent during
	// the test run.
	ExpectedTraces() trace.Traces
}

func RunTest(t *testing.T, tc TestCase) {
	t.Helper()
	require.True(t, built.WithOrchestrion, "this test suite must be run with orchestrion enabled")

	mockAgent, err := agent.New(t)
	require.NoError(t, err)
	defer mockAgent.Close()

	t.Log("Running setup")
	tc.Setup(t)

	sess, err := mockAgent.NewSession(t)
	require.NoError(t, err)

	t.Log("Running test")
	tc.Run(t)

	checkTraces(t, tc, sess)
}

func checkTraces(t *testing.T, tc TestCase, sess *agent.Session) {
	t.Helper()

	jsonTraces, err := sess.Close(t)
	require.NoError(t, err)

	var got trace.Traces
	require.NoError(t, trace.ParseRaw(jsonTraces, &got))
	t.Logf("Received %d traces", len(got))
	for i, tr := range got {
		t.Logf("[%d] Trace contains a total of %d spans:\n%v", i, tr.NumSpans(), tr)
	}

	for _, expected := range tc.ExpectedTraces() {
		expected.RequireAnyMatch(t, got)
	}
}
