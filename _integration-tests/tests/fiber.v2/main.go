// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package fiber

import (
	"net/http"
	"orchestrion/integration/validator/trace"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/require"
)

type TestCase struct {
	*fiber.App
}

func (tc *TestCase) Setup(t *testing.T) {
	tc.App = fiber.New(fiber.Config{DisableStartupMessage: true})
	tc.App.Get("/ping", func(c *fiber.Ctx) error { return c.JSON(map[string]any{"message": "pong"}) })
	go func() { require.NoError(t, tc.App.Listen("127.0.0.1:8080")) }()
}

func (tc *TestCase) Run(t *testing.T) {
	resp, err := http.Get("http://127.0.0.1:8080/ping")
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)
}

func (tc *TestCase) Teardown(t *testing.T) {
	require.NoError(t, tc.App.ShutdownWithTimeout(time.Second))
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
			Meta: map[string]any{
				"http.url": "http://127.0.0.1:8080/ping",
			},
			Children: trace.Spans{
				{
					Tags: map[string]any{
						"name":     "http.request",
						"service":  "fiber",
						"resource": "GET /ping",
						"type":     "web",
					},
					Meta: map[string]any{
						"http.url": "/ping",
					},
				},
			},
		},
	}
}
