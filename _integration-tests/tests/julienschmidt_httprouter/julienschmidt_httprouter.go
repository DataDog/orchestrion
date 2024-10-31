// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

//go:build integration

package julienschmidt_httprouter

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/julienschmidt/httprouter"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"datadoghq.dev/orchestrion/_integration-tests/utils"
	"datadoghq.dev/orchestrion/_integration-tests/validator/trace"
)

type TestCase struct {
	*http.Server
}

func (tc *TestCase) Setup(t *testing.T) {
	router := httprouter.New()
	router.GET("/ping", func(w http.ResponseWriter, _ *http.Request, _ httprouter.Params) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, err := io.WriteString(w, `{"message": "pong"}`)
		assert.NoError(t, err)
	})
	tc.Server = &http.Server{
		Addr:         "127.0.0.1:" + utils.GetFreePort(t),
		Handler:      router,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
	}
	go func() { assert.ErrorIs(t, tc.Server.ListenAndServe(), http.ErrServerClosed) }()
}

func (tc *TestCase) Run(t *testing.T) {
	resp, err := http.Get(fmt.Sprintf("http://%s/ping", tc.Server.Addr))
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)
}

func (tc *TestCase) Teardown(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	require.NoError(t, tc.Server.Shutdown(ctx))
}

func (*TestCase) ExpectedTraces() trace.Traces {
	return trace.Traces{
		{
			Tags: map[string]any{
				"name":     "http.request",
				"resource": "GET /ping",
				"type":     "http",
				"service":  "julienschmidt_httprouter.test",
			},
			Meta: map[string]string{
				"component": "net/http",
				"span.kind": "client",
			},
			Children: trace.Traces{
				{
					Tags: map[string]any{
						"name":     "http.request",
						"resource": "GET /ping",
						"type":     "web",
						"service":  "julienschmidt_httprouter.test",
					},
					Meta: map[string]string{
						"component": "net/http",
						"span.kind": "server",
					},
					Children: trace.Traces{
						{
							Tags: map[string]any{
								"name":     "http.request",
								"resource": "GET /ping",
								"type":     "web",
								"service":  "http.router",
							},
							Meta: map[string]string{
								"component": "julienschmidt/httprouter",
								"span.kind": "server",
							},
						},
					},
				},
			},
		},
	}
}
