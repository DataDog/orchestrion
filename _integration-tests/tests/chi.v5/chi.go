// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

//go:build integration

package chiv5

import (
	"context"
	"fmt"
	"net/http"
	"orchestrion/integration/validator/trace"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/require"
)

type TestCase struct {
	*http.Server
}

func (tc *TestCase) Setup(t *testing.T) {
	router := chi.NewRouter()

	//dd:ignore
	tc.Server = &http.Server{
		Addr:    "127.0.0.1:8080",
		Handler: router,
	}

	router.Get("/", func(w http.ResponseWriter, _ *http.Request) {
		w.Write([]byte("Hello World!\n"))
	})

	go func() {
		require.ErrorIs(t, tc.Server.ListenAndServe(), http.ErrServerClosed)
	}()
}

func (tc *TestCase) Run(t *testing.T) {
	resp, err := http.Get(fmt.Sprintf("http://%s/", tc.Server.Addr))
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)
}

func (tc *TestCase) Teardown(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	require.NoError(t, tc.Server.Shutdown(ctx))
}

func (tc *TestCase) ExpectedTraces() trace.Spans {
	return trace.Spans{
		{
			// NB: Top-level span is from the HTTP Client, which is library-side instrumented.
			Tags: map[string]any{
				"name":     "http.request",
				"resource": "GET /",
				"type":     "http",
			},
			Meta: map[string]any{
				"http.url": fmt.Sprintf("http://%s/", tc.Server.Addr),
			},
			Children: trace.Spans{
				{
					Tags: map[string]any{
						"name":     "http.request",
						"resource": "GET /",
						"service":  "chi.router",
						"type":     "web",
					},
					Meta: map[string]any{
						"http.url": fmt.Sprintf("http://%s/", tc.Server.Addr),
					},
				},
			},
		},
	}
}
