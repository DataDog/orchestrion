// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

//go:build integration

package nethttp

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"orchestrion/integration/validator/trace"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type TestCase struct {
	*http.Server
}

func (tc *TestCase) Setup(t *testing.T) {
	mux := http.NewServeMux()
	tc.Server = &http.Server{
		Addr:    "127.0.0.1:8080",
		Handler: mux,
	}

	mux.HandleFunc("/hit", tc.handleHit)
	mux.HandleFunc("/", tc.handleRoot)

	go func() { require.ErrorIs(t, tc.Server.ListenAndServe(), http.ErrServerClosed) }()
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
						"component": "net/http",
						"span.kind": "server",
					},
					Children: trace.Spans{
						{
							Tags: map[string]any{
								"name":     "http.request",
								"resource": "POST /hit",
								"type":     "http",
							},
							Meta: map[string]any{
								"http.url":                 fmt.Sprintf("http://%s/hit", tc.Server.Addr),
								"component":                "net/http",
								"span.kind":                "client",
								"network.destination.name": "127.0.0.1",
								"http.status_code":         "200",
								"http.method":              "POST",
							},
							Children: trace.Spans{
								{
									Tags: map[string]any{
										"name":     "http.request",
										"resource": "POST /hit",
										"type":     "web",
									},
									Meta: map[string]any{
										"http.useragent":   "Go-http-client/1.1",
										"http.status_code": "200",
										"http.host":        tc.Server.Addr,
										"component":        "net/http",
										"http.url":         fmt.Sprintf("http://%s/hit", tc.Server.Addr),
										"http.method":      "POST",
										"span.kind":        "server",
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

func (tc *TestCase) handleRoot(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()

	resp, err := http.Post(fmt.Sprintf("http://%s/hit", tc.Server.Addr), "text/plain", r.Body)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(err.Error()))
		return
	}
	defer resp.Body.Close()

	b, err := io.ReadAll(resp.Body)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(err.Error()))
		return
	}

	w.WriteHeader(resp.StatusCode)
	w.Write(b)
}

func (*TestCase) handleHit(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()

	b, err := io.ReadAll(r.Body)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(err.Error()))
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Write(b)
}
