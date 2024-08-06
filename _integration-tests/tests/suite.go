// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package tests

import (
	"testing"

	"orchestrion/integration/validator/trace"
)

//go:generate go run ../utils/generator .

// testCase describes the general contract for tests. Each package in this
// directory is expected to export a `TestCase` structure implementing this
// interface.
type testCase interface {
	// Setup is called before the test is run. It should be used to prepare any
	// the test for execution, such as starting up services (e.g, databse servers)
	// or setting up test data. The Setup function can call `t.SkipNow()` to skip
	// the test entirely, for example if prerequisites of its dependencies are not
	// satisfied by the test environment.
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

	// Teardown runs if `Setup` was executed successfully and did not call
	// `t.SkipNow()`. This can be used to clean up any resources created during
	// Setup, such as stopping services or deleting test data.
	Teardown(*testing.T)

	// ExpectedTraces returns a trace.Spans object describing all traces expected
	// to be produced by the `Run` function. There should be one entry per trace
	// root span expected to be produced. Every item in the returned `trace.Spans`
	// must match at least one trace received by the agent during the test run.
	ExpectedTraces() trace.Spans
}
