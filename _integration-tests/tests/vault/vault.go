// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

//go:build integration

package vault

import (
	"context"
	"testing"
	"time"

	"datadoghq.dev/orchestrion/_integration-tests/utils"
	"datadoghq.dev/orchestrion/_integration-tests/validator/trace"
	"github.com/hashicorp/vault/api"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	testvault "github.com/testcontainers/testcontainers-go/modules/vault"
	"gopkg.in/DataDog/dd-trace-go.v1/ddtrace/tracer"
)

type TestCase struct {
	server *testvault.VaultContainer
	*api.Client
}

func (tc *TestCase) Setup(t *testing.T) {
	ctx := context.Background()

	var err error
	tc.server, err = testvault.Run(ctx,
		"vault:1.7.3",
		testcontainers.WithLogger(testcontainers.TestLogger(t)),
		utils.WithTestLogConsumer(t),
		testvault.WithToken("root"),
	)
	utils.AssertTestContainersError(t, err)

	addr, err := tc.server.HttpHostAddress(ctx)
	if err != nil {
		defer tc.server.Terminate(ctx)
		t.Skipf("Failed to get vault container address: %v\n", err)
	}
	c, err := api.NewClient(&api.Config{
		Address: addr,
	})
	c.SetToken("root")
	if err != nil {
		t.Fatal(err)
	}
	tc.Client = c
}

func (tc *TestCase) Run(t *testing.T) {
	ctx := context.Background()
	span, ctx := tracer.StartSpanFromContext(ctx, "test.root")
	defer span.Finish()

	_, err := tc.Logical().ReadWithContext(ctx, "secret/key")
	require.NoError(t, err)
}

func (tc *TestCase) Teardown(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	require.NoError(t, tc.server.Terminate(ctx))
}

func (*TestCase) ExpectedTraces() trace.Traces {
	return trace.Traces{
		{
			Tags: map[string]any{
				"name": "test.root",
			},
			Children: trace.Traces{
				{
					Tags: map[string]any{
						"name":     "http.request",
						"service":  "vault",
						"resource": "GET /v1/secret/key",
						"type":     "http",
					},
					Meta: map[string]string{
						"http.method": "GET",
						"http.url":    "/v1/secret/key",
						"span.kind":   "client",
					},
				},
			},
		},
	}
}
