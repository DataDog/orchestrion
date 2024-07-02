// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package echo

import (
	"context"
	"io"
	"net/http"
	"orchestrion/integration/validator/trace"
	"testing"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/require"
)

type TestCase struct {
	*echo.Echo
}

func (tc *TestCase) Setup(t *testing.T) {
	tc.Echo = echo.New()
	tc.Echo.Logger.SetOutput(io.Discard)

	tc.Echo.GET("/ping", func(c echo.Context) error {
		return c.JSON(http.StatusOK, map[string]any{"message": "pong"})
	})

	go func() { require.ErrorIs(t, tc.Echo.Start("127.0.0.1:8080"), http.ErrServerClosed) }()
}

func (tc *TestCase) Run(t *testing.T) {
	resp, err := http.Get("http://127.0.0.1:8080/ping")
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)
}

func (tc *TestCase) Teardown(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	require.NoError(t, tc.Echo.Shutdown(ctx))
}

func (*TestCase) ExpectedTraces() trace.Spans {
	return trace.Spans{
		{
			// NB: Top-level span is from the HTTP Client, which is library-side instrumented.
			Tags: map[string]any{
				"name":     "http.request",
				"resource": "GET /ping",
				"service":  "tests.test",
				"type":     "http",
			},
			Children: trace.Spans{
				{
					Tags: map[string]any{
						"name":     "http.request",
						"service":  "echo",
						"resource": "GET /ping",
						"type":     "web",
					},
					Meta: map[string]any{
						"http.url": "http://127.0.0.1:8080/ping",
					},
				},
			},
		},
	}
}
