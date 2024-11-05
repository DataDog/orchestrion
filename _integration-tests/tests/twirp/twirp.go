// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

//go:build integration

package twirp

import (
	"context"
	"net"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/twitchtv/twirp"
	"github.com/twitchtv/twirp/example"

	"datadoghq.dev/orchestrion/_integration-tests/validator/trace"
)

type TestCase struct {
	srv    *http.Server
	client example.Haberdasher
	addr   string
}

func (tc *TestCase) Setup(t *testing.T) {
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)

	tc.addr = "http://" + lis.Addr().String()
	handler := example.NewHaberdasherServer(&randomHaberdasher{})
	tc.srv = &http.Server{Handler: handler}

	go func() {
		assert.ErrorIs(t, tc.srv.Serve(lis), http.ErrServerClosed)
	}()
	tc.client = example.NewHaberdasherJSONClient(tc.addr, http.DefaultClient)
}

func (tc *TestCase) Run(t *testing.T) {
	ctx := context.Background()
	_, err := tc.client.MakeHat(ctx, &example.Size{Inches: 6})
	require.NoError(t, err)
}

func (tc *TestCase) Teardown(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	require.NoError(t, tc.srv.Shutdown(ctx))
}

func (*TestCase) ExpectedTraces() trace.Traces {
	return trace.Traces{
		{
			Tags: map[string]any{
				"name":     "http.request",
				"service":  "twirp.test",
				"resource": "POST /twirp/twitch.twirp.example.Haberdasher/MakeHat",
				"type":     "http",
			},
			Meta: map[string]string{
				"component": "net/http",
				"span.kind": "client",
			},
			Children: trace.Traces{
				{
					Tags: map[string]any{
						"name":     "http.request",
						"service":  "twirp.test",
						"resource": "POST /twirp/twitch.twirp.example.Haberdasher/MakeHat",
						"type":     "web",
					},
					Meta: map[string]string{
						"component": "net/http",
						"span.kind": "server",
					},
					Children: trace.Traces{
						{
							Tags: map[string]any{
								"name":     "twirp.Haberdasher",
								"service":  "twirp-server",
								"resource": "MakeHat",
								"type":     "web",
							},
							Meta: map[string]string{
								"component":     "twitchtv/twirp",
								"rpc.system":    "twirp",
								"rpc.service":   "Haberdasher",
								"rpc.method":    "MakeHat",
								"twirp.method":  "MakeHat",
								"twirp.package": "twitch.twirp.example",
								"twirp.service": "Haberdasher",
							},
						},
					},
				},
			},
		},
	}
}

type randomHaberdasher struct{}

func (*randomHaberdasher) MakeHat(_ context.Context, size *example.Size) (*example.Hat, error) {
	if size.Inches <= 0 {
		return nil, twirp.InvalidArgumentError("Inches", "I can't make a hat that small!")
	}
	return &example.Hat{
		Size:  size.Inches,
		Color: "blue",
		Name:  "top hat",
	}, nil
}
