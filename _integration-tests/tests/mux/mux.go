// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

//go:build integration

package mux

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"testing"
	"time"

	"datadoghq.dev/orchestrion/_integration-tests/utils"
	"datadoghq.dev/orchestrion/_integration-tests/validator/trace"
	"github.com/gorilla/mux"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type TestCase struct {
	*http.Server
}

func (tc *TestCase) Setup(t *testing.T) {
	router := mux.NewRouter()
	tc.Server = &http.Server{
		Addr:    "127.0.0.1:" + utils.GetFreePort(t),
		Handler: router,
	}
	router.HandleFunc("/ping", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, err := io.WriteString(w, `{"message": "pong"}`)
		assert.NoError(t, err)
	}).Methods("GET")

	go func() { assert.ErrorIs(t, tc.Server.ListenAndServe(), http.ErrServerClosed) }()
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		assert.NoError(t, tc.Server.Shutdown(ctx))
	})
}

func (tc *TestCase) Run(t *testing.T) {
	resp, err := http.Get(fmt.Sprintf("http://%s/ping", tc.Server.Addr))
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)
}

func (tc *TestCase) ExpectedTraces() trace.Traces {
	url := fmt.Sprintf("http://%s/ping", tc.Server.Addr)
	return trace.Traces{
		{
			// NB: Top-level span is from the HTTP Client, which is library-side instrumented.
			Tags: map[string]any{
				"name":     "http.request",
				"resource": "GET /ping",
				"type":     "http",
				"service":  "mux.test",
			},
			Meta: map[string]string{
				"http.url":  url,
				"component": "net/http",
				"span.kind": "client",
			},
			Children: trace.Traces{
				{
					Tags: map[string]any{
						"name":     "http.request",
						"resource": "GET /ping",
						"type":     "web",
						"service":  "mux.test",
					},
					Meta: map[string]string{
						"http.url":  url,
						"component": "net/http",
						"span.kind": "server",
					},
					Children: trace.Traces{
						{
							Tags: map[string]any{
								"name":     "http.request",
								"resource": "GET /ping",
								"type":     "web",
								"service":  "mux.router",
							},
							Meta: map[string]string{
								"http.url":  url,
								"component": "gorilla/mux",
								"span.kind": "server",
							},
							Children: trace.Traces{
								{
									// FIXME: this span shouldn't exist
									Tags: map[string]any{
										"name":     "http.request",
										"resource": "GET /ping",
										"type":     "web",
										"service":  "mux.router",
									},
									Meta: map[string]string{
										"http.url":  url,
										"component": "net/http",
										"span.kind": "server",
									},
								},
							},
						},
					},
				},
			},
		},
	}
}
