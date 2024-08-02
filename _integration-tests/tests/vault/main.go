// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package vault

import (
	"context"
	"fmt"
	"testing"
	"time"

	"orchestrion/integration/validator/trace"

	"github.com/hashicorp/vault/api"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	testvault "github.com/testcontainers/testcontainers-go/modules/vault"
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
		testcontainers.WithLogConsumers(testLogConsumer{t}),
		testvault.WithToken("root"),
	)
	if err != nil {
		t.Skipf("Failed to start vault test container: %v\n", err)
	}

	serverIP, err := tc.server.ContainerIP(ctx)
	if err != nil {
		t.Skipf("Failed to get vault container IP: %v\n", err)
	}

	c, err := api.NewClient(&api.Config{
		Address: fmt.Sprintf("http://%s:8200", serverIP),
	})
	c.SetToken("root")
	if err != nil {
		t.Fatal(err)
	}
	tc.Client = c
}

func (tc *TestCase) Run(t *testing.T) {
	_, err := tc.Logical().Read("secret/key")
	require.NoError(t, err)
}

func (tc *TestCase) Teardown(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	require.NoError(t, tc.server.Terminate(ctx))
}

func (tc *TestCase) ExpectedTraces() trace.Spans {
	return trace.Spans{
		{
			Tags: map[string]interface{}{
				"service": "vault",
			},
		},
	}
}

type testLogConsumer struct {
	*testing.T
}

func (t testLogConsumer) Accept(log testcontainers.Log) {
	t.T.Log(string(log.Content))
}
