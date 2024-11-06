// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

//go:build integration

package goredis

import (
	"context"
	"fmt"
	"testing"
	"time"

	"datadoghq.dev/orchestrion/_integration-tests/utils"
	"datadoghq.dev/orchestrion/_integration-tests/validator/trace"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	testredis "github.com/testcontainers/testcontainers-go/modules/redis"
	"gopkg.in/DataDog/dd-trace-go.v1/ddtrace/tracer"
)

type TestCase struct {
	server *testredis.RedisContainer
	*redis.Client
	key string
}

func (tc *TestCase) Setup(t *testing.T) {
	utils.SkipIfProviderIsNotHealthy(t)

	uuid, err := uuid.NewRandom()
	require.NoError(t, err)
	tc.key = uuid.String()

	container, addr := utils.StartRedisTestContainer(t)
	tc.server = container

	tc.Client = redis.NewClient(&redis.Options{Addr: addr})
}

func (tc *TestCase) Run(t *testing.T) {
	span, ctx := tracer.StartSpanFromContext(context.Background(), "test.root")
	defer span.Finish()

	require.NoError(t, tc.Client.Set(ctx, tc.key, "test_value", 0).Err())
	require.NoError(t, tc.Client.Get(ctx, tc.key).Err())
}

func (tc *TestCase) Teardown(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	assert.NoError(t, tc.Client.Close())
	if tc.server != nil && assert.NoError(t, tc.server.Terminate(ctx)) {
		tc.server = nil
	}
}

func (tc *TestCase) ExpectedTraces() trace.Traces {
	return trace.Traces{
		{
			Tags: map[string]any{
				"name": "test.root",
			},
			Children: trace.Traces{
				{
					Tags: map[string]any{
						"name":     "redis.command",
						"service":  "redis.client",
						"resource": "set",
						"type":     "redis",
					},
					Meta: map[string]string{
						"redis.args_length": "3",
						"component":         "redis/go-redis.v9",
						"out.db":            "0",
						"span.kind":         "client",
						"db.system":         "redis",
						"redis.raw_command": fmt.Sprintf("set %s test_value: ", tc.key),
						"out.host":          "localhost",
					},
				},
				{
					Tags: map[string]any{
						"name":     "redis.command",
						"service":  "redis.client",
						"resource": "get",
						"type":     "redis",
					},
					Meta: map[string]string{
						"redis.args_length": "2",
						"component":         "redis/go-redis.v9",
						"out.db":            "0",
						"span.kind":         "client",
						"db.system":         "redis",
						"redis.raw_command": fmt.Sprintf("get %s: ", tc.key),
						"out.host":          "localhost",
					},
				},
			},
		},
	}
}
