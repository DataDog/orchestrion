// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

//go:build integration

package os

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"testing"
	"time"

	"orchestrion/integration/validator/trace"

	"github.com/stretchr/testify/require"

	"gopkg.in/DataDog/dd-trace-go.v1/appsec/events"
	"gopkg.in/DataDog/dd-trace-go.v1/ddtrace/tracer"
)

type TestCase struct {
	*http.Server
	*testing.T
}

func (tc *TestCase) Setup(t *testing.T) {
	t.Setenv("DD_APPSEC_RULES", "./testdata/rasp-only-rules.json")
	t.Setenv("DD_APPSEC_ENABLED", "true")
	t.Setenv("DD_APPSEC_RASP_ENABLED", "true")
	t.Setenv("DD_APPSEC_WAF_TIMEOUT", "1h")
	mux := http.NewServeMux()
	tc.Server = &http.Server{
		Addr:    "127.0.0.1:8080",
		Handler: mux,
	}

	mux.HandleFunc("/", tc.handleRoot)

	go func() { require.ErrorIs(t, tc.Server.ListenAndServe(), http.ErrServerClosed) }()
}

func (tc *TestCase) Run(t *testing.T) {
	tc.T = t
	resp, err := http.Get(fmt.Sprintf("http://%s/?path=/etc/passwd", tc.Server.Addr))
	require.NoError(t, err)
	require.Equal(t, http.StatusForbidden, resp.StatusCode)
}

func (tc *TestCase) Teardown(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	require.NoError(t, tc.Server.Shutdown(ctx))
}

func (tc *TestCase) ExpectedTraces() trace.Spans {
	return trace.Spans{
		{
			Tags: map[string]any{
				"name":     "http.request",
				"resource": "GET /",
				"type":     "http",
			},
			Meta: map[string]any{
				"component": "net/http",
				"span.kind": "client",
			},
			Children: trace.Spans{
				{
					Tags: map[string]any{
						"name":     "http.request",
						"resource": "GET /",
						"type":     "web",
					},
					Meta: map[string]any{
						"component":         "net/http",
						"span.kind":         "server",
						"appsec.blocked":    "true",
						"is.security.error": "true",
					},
				},
			},
		},
	}
}

func (tc *TestCase) handleRoot(w http.ResponseWriter, _ *http.Request) {

	fp, err := os.Open("/etc/passwd")

	require.ErrorIs(tc.T, err, &events.BlockingSecurityEvent{})
	if events.IsSecurityError(err) { // TODO: response writer instrumentation to not have to do that
		span, _ := tracer.SpanFromContext(context.TODO())
		span.SetTag("is.security.error", true)
		return
	}

	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	defer fp.Close()

	w.WriteHeader(http.StatusOK)
}
