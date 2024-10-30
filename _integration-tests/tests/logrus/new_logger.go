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

type TestCaseNewLogger struct {
	logger *logrus.Logger
	logs   *bytes.Buffer
}

func (tc *TestCaseNewLogger) Setup(*testing.T) {
	tc.logs = new(bytes.Buffer)
	tc.logger = logrus.New()
	tc.logger.SetLevel(logrus.DebugLevel)
	tc.logger.SetOutput(tc.logs)
}

func (tc *TestCaseNewLogger) Run(t *testing.T) {
	runTest(t, tc.logs, tc.Log)
}

func (tc *TestCaseNewLogger) Teardown(*testing.T) {}

func (*TestCaseNewLogger) ExpectedTraces() trace.Traces {
	return expectedTraces()
}

//dd:span
func (tc *TestCaseNewLogger) Log(ctx context.Context, level logrus.Level, msg string) {
	tc.logger.WithContext(ctx).Log(level, msg)
}
