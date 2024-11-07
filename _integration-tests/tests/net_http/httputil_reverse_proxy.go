// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

//go:build integration

package nethttp

import (
	"context"
	"net/http"
	"net/http/httputil"
	"net/url"
	"testing"
	"time"

	"datadoghq.dev/orchestrion/_integration-tests/utils"
	"datadoghq.dev/orchestrion/_integration-tests/validator/trace"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type TestCaseReverseProxy struct {
	proxy    *http.Server
	upstream *http.Server
}

func (tc *TestCaseReverseProxy) Setup(t *testing.T) {
	tc.upstream = &http.Server{
		Addr: "127.0.0.1:" + utils.GetFreePort(t),
		Handler: http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusOK)
		}),
	}
	target, err := url.Parse(rootUrl(tc.upstream))
	require.NoError(t, err)

	go func() { assert.ErrorIs(t, tc.upstream.ListenAndServe(), http.ErrServerClosed) }()
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		assert.NoError(t, tc.upstream.Shutdown(ctx))
	})

	proxy := httputil.NewSingleHostReverseProxy(target)
	tc.proxy = &http.Server{
		Addr:    "127.0.0.1:" + utils.GetFreePort(t),
		Handler: proxy,
	}

	go func() { assert.ErrorIs(t, tc.proxy.ListenAndServe(), http.ErrServerClosed) }()
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		assert.NoError(t, tc.proxy.Shutdown(ctx))
	})
}

func (tc *TestCaseReverseProxy) Run(t *testing.T) {
	resp, err := http.Get(rootUrl(tc.proxy))
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)
}

func (tc *TestCaseReverseProxy) ExpectedTraces() trace.Traces {
	return trace.Traces{
		{
			Tags: map[string]any{
				"name":     "http.request",
				"resource": "GET /",
				"type":     "http",
			},
			Meta: map[string]string{
				"component": "net/http",
				"span.kind": "client",
				"http.url":  rootUrl(tc.proxy),
			},
			Children: trace.Traces{
				{
					Tags: map[string]any{
						"name":     "http.request",
						"resource": "GET /",
						"type":     "web",
					},
					Meta: map[string]string{
						"component": "net/http",
						"span.kind": "server",
						"http.url":  rootUrl(tc.proxy),
					},
					Children: trace.Traces{
						{
							Tags: map[string]any{
								"name":     "http.request",
								"resource": "GET /",
								"type":     "http",
							},
							Meta: map[string]string{
								"component": "net/http",
								"span.kind": "client",
								"http.url":  rootUrl(tc.upstream),
							},
							Children: trace.Traces{
								{
									Tags: map[string]any{
										"name":     "http.request",
										"resource": "GET /",
										"type":     "web",
									},
									Meta: map[string]string{
										"component": "net/http",
										"span.kind": "server",
										"http.url":  rootUrl(tc.upstream),
									},
									Children: nil,
								},
							},
						},
					},
				},
			},
		},
	}
}

func rootUrl(srv *http.Server) string {
	return "http://" + srv.Addr + "/"
}
