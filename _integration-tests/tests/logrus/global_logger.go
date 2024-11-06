// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

//go:build integration

package logrus

import (
	"bytes"
	"context"
	"testing"

	"github.com/sirupsen/logrus"

	"datadoghq.dev/orchestrion/_integration-tests/validator/trace"
)

type TestCaseGlobalLogger struct {
	logs *bytes.Buffer
}

func (tc *TestCaseGlobalLogger) Setup(*testing.T) {
	tc.logs = new(bytes.Buffer)
	logrus.SetLevel(logrus.DebugLevel)
	logrus.SetOutput(tc.logs)
}

func (tc *TestCaseGlobalLogger) Run(t *testing.T) {
	runTest(t, tc.logs, tc.Log)
}

func (*TestCaseGlobalLogger) Teardown(*testing.T) {}

func (*TestCaseGlobalLogger) ExpectedTraces() trace.Traces {
	return expectedTraces()
}

//dd:span
func (*TestCaseGlobalLogger) Log(ctx context.Context, level logrus.Level, msg string) {
	logrus.WithContext(ctx).Log(level, msg)
}
