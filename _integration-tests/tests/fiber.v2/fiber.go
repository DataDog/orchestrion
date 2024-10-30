// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

//go:build integration

package fiber

import (
	"net/http"
	"testing"
	"time"

	"datadoghq.dev/orchestrion/_integration-tests/utils"
	"datadoghq.dev/orchestrion/_integration-tests/validator/trace"
	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type TestCase struct {
	*fiber.App
	addr string
}

func (tc *TestCase) Setup(t *testing.T) {
	tc.App = fiber.New(fiber.Config{DisableStartupMessage: true})
	tc.App.Get("/ping", func(c *fiber.Ctx) error { return c.JSON(map[string]any{"message": "pong"}) })
	tc.addr = "127.0.0.1:" + utils.GetFreePort(t)

	go func() { assert.NoError(t, tc.App.Listen(tc.addr)) }()
}

func (tc *TestCase) Run(t *testing.T) {
	resp, err := http.Get("http://" + tc.addr + "/ping")
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)
}

func (tc *TestCase) Teardown(t *testing.T) {
	require.NoError(t, tc.App.ShutdownWithTimeout(time.Second))
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
			Meta: map[string]string{
				"http.url": "http://" + tc.addr + "/ping",
			},
			Children: trace.Traces{
				{
					Tags: map[string]any{
						"name":     "http.request",
						"service":  "fiber",
						"resource": "GET /ping",
						"type":     "web",
					},
					Meta: map[string]string{
						"http.url": "/ping",
					},
				},
			},
		},
	}
}
