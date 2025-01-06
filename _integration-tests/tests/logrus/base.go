// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

//go:build integration

package logrus

import (
	"bufio"
	"bytes"
	"context"
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"gopkg.in/DataDog/dd-trace-go.v1/ddtrace/tracer"

	"datadoghq.dev/orchestrion/_integration-tests/validator/trace"
)

func runTest(ctx context.Context, t *testing.T, out *bytes.Buffer, logFn func(context.Context, logrus.Level, string)) {
	span, ctx := tracer.StartSpanFromContext(ctx, "test.root")
	defer span.Finish()

	logFn(ctx, logrus.DebugLevel, "debug")
	logFn(ctx, logrus.InfoLevel, "info")
	logFn(ctx, logrus.WarnLevel, "warn")
	logFn(ctx, logrus.ErrorLevel, "error")

	logs := out.String()
	t.Logf("got logs: %s", logs)
	for _, msg := range []string{"debug", "info", "warn", "error"} {
		want := "msg=" + msg
		assert.Contains(t, logs, want, "missing log message")
	}

	s := bufio.NewScanner(out)
	for s.Scan() {
		line := string(s.Bytes())
		t.Logf("%s", line)
		assert.Regexp(t, `dd.span_id=\d+`, line, "no span ID")
		assert.Regexp(t, `dd.trace_id=\d+`, line, "no trace ID")
	}
}

func expectedTraces() trace.Traces {
	return trace.Traces{
		{
			Tags: map[string]any{
				"name": "test.root",
			},
			Children: trace.Traces{
				{
					Tags: map[string]any{
						"name": "Log",
					},
				},
				{
					Tags: map[string]any{
						"name": "Log",
					},
				},
				{
					Tags: map[string]any{
						"name": "Log",
					},
				},
				{
					Tags: map[string]any{
						"name": "Log",
					},
				},
			},
		},
	}
}
