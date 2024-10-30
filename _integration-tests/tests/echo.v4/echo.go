// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

//go:build integration

package echo

import (
	"context"
	"io"
	"net/http"
	"testing"
	"time"

	"datadoghq.dev/orchestrion/_integration-tests/utils"
	"datadoghq.dev/orchestrion/_integration-tests/validator/trace"
	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type TestCase struct {
	*echo.Echo
	addr string
}

func (tc *TestCase) Setup(t *testing.T) {
	tc.Echo = echo.New()
	tc.Echo.Logger.SetOutput(io.Discard)

	tc.Echo.GET("/ping", func(c echo.Context) error {
		return c.JSON(http.StatusOK, map[string]any{"message": "pong"})
	})
	tc.addr = "127.0.0.1:" + utils.GetFreePort(t)

	go func() { assert.ErrorIs(t, tc.Echo.Start(tc.addr), http.ErrServerClosed) }()
}

func (tc *TestCase) Run(t *testing.T) {
	resp, err := http.Get("http://" + tc.addr + "/ping")
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)
}

func (tc *TestCase) Teardown(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	require.NoError(t, tc.Echo.Shutdown(ctx))
}

func (tc *TestCase) ExpectedTraces() trace.Traces {
	return trace.Traces{
		{
			// NB: Top-level span is from the HTTP Client, which is library-side instrumented.
			Tags: map[string]any{
				"name":     "http.request",
				"resource": "GET /ping",
				"type":     "http",
			},
			Children: trace.Traces{
				{
					Tags: map[string]any{
						"name":     "http.request",
						"service":  "echo",
						"resource": "GET /ping",
						"type":     "web",
					},
					Meta: map[string]string{
						"http.url": "http://" + tc.addr + "/ping",
					},
				},
			},
		},
	}
}
