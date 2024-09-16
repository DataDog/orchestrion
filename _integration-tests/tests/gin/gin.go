// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

//go:build integration

package gin

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"orchestrion/integration/utils"
	"orchestrion/integration/validator/trace"
)

type TestCase struct {
	*http.Server
}

func (tc *TestCase) Setup(t *testing.T) {
	gin.SetMode(gin.ReleaseMode) // Silence start-up logging
	engine := gin.New()

	tc.Server = &http.Server{
		Addr:    "127.0.0.1:" + utils.GetFreePort(t),
		Handler: engine.Handler(),
	}

	engine.GET("/ping", func(c *gin.Context) { c.JSON(http.StatusOK, gin.H{"message": "pong"}) })

	go func() { assert.ErrorIs(t, tc.Server.ListenAndServe(), http.ErrServerClosed) }()
}

func (tc *TestCase) Run(t *testing.T) {
	resp, err := http.Get("http://" + tc.Server.Addr + "/ping")
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)
}

func (tc *TestCase) Teardown(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	require.NoError(t, tc.Server.Shutdown(ctx))
}

func (*TestCase) ExpectedTraces() trace.Spans {
	return trace.Spans{
		{
			// NB: Top-level span is from the HTTP Client, which is library-side instrumented.
			Tags: map[string]any{
				"name":     "http.request",
				"resource": "GET /ping",
				"type":     "http",
			},
			Meta: map[string]any{
				"http.url": "http://127.0.0.1:8080/ping",
			},
			Children: trace.Spans{
				{
					Tags: map[string]any{
						"name":     "http.request",
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
