// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package slog

import (
	"bufio"
	"bytes"
	"context"
	"log/slog"
	"regexp"
	"strings"
	"testing"

	"orchestrion/integration/validator/trace"
)

type TestCase struct {
	logger *slog.Logger
	logs   *bytes.Buffer
}

func (tc *TestCase) Setup(t *testing.T) {
	tc.logs = new(bytes.Buffer)
	tc.logger = slog.New(
		slog.NewTextHandler(
			tc.logs,
			&slog.HandlerOptions{Level: slog.LevelDebug},
		),
	)
}

//dd:span
func Log(ctx context.Context, f func(context.Context, string, ...any), msg string) {
	f(ctx, msg)
}

func (tc *TestCase) Run(t *testing.T) {
	Log(context.Background(), tc.logger.DebugContext, "debug")
	Log(context.Background(), tc.logger.InfoContext, "info")
	Log(context.Background(), tc.logger.WarnContext, "warn")
	Log(context.Background(), tc.logger.ErrorContext, "error")
	Log(context.Background(), func(ctx context.Context, s string, a ...any) {
		tc.logger.Log(ctx, slog.LevelInfo, s, a...)
	}, "log")

	logs := tc.logs.String()
	t.Logf("got logs: %s", logs)
	for _, msg := range []string{"debug", "info", "warn", "error", "log"} {
		want := "msg=" + msg
		if !strings.Contains(logs, want) {
			t.Fatalf("missing log message %s", msg)
		}
	}

	s := bufio.NewScanner(tc.logs)
	for s.Scan() {
		line := s.Bytes()
		t.Logf("%s", line)
		if ok, _ := regexp.Match(`dd.span_id=\d+`, line); !ok {
			t.Errorf("no span ID")
		}
		if ok, _ := regexp.Match(`dd.trace_id=\d+`, line); !ok {
			t.Errorf("no trace ID")
		}
	}
}

func (tc *TestCase) Teardown(t *testing.T) {}

func (*TestCase) ExpectedTraces() trace.Spans { return trace.Spans{} }
