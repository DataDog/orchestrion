// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

//go:build integration

package goredis

import (
	"context"
	"fmt"
	"log"
	"net/url"
	"testing"
	"time"

	"datadoghq.dev/orchestrion/_integration-tests/utils"
	"datadoghq.dev/orchestrion/_integration-tests/validator/trace"
	"github.com/go-redis/redis"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	testredis "github.com/testcontainers/testcontainers-go/modules/redis"
	"github.com/testcontainers/testcontainers-go/wait"
	"gopkg.in/DataDog/dd-trace-go.v1/ddtrace/tracer"
)

type TestCase struct {
	server *testredis.RedisContainer
	*redis.Client
	key string
}

func (tc *TestCase) Setup(t *testing.T) {
	utils.SkipIfProviderIsNotHealthy(t)

	ctx := context.Background()

	uuid, err := uuid.NewRandom()
	require.NoError(t, err)
	tc.key = uuid.String()

	tc.server, err = testredis.Run(ctx,
		"redis:7",
		testcontainers.WithLogger(testcontainers.TestLogger(t)),
		utils.WithTestLogConsumer(t),
		testcontainers.WithWaitStrategy(
			wait.ForAll(
				wait.ForLog("* Ready to accept connections"),
				wait.ForExposedPort(),
				wait.ForListeningPort("6379/tcp"),
			),
		),
	)
	utils.AssertTestContainersError(t, err)

	redisURI, err := tc.server.ConnectionString(ctx)
	if err != nil {
		log.Fatalf("Failed to obtain connection string: %v\n", err)
	}
	redisURL, err := url.Parse(redisURI)
	if err != nil {
		log.Fatalf("Invalid redis connection string: %q\n", redisURI)
	}
	addr := redisURL.Host

	tc.Client = redis.NewClient(&redis.Options{Addr: addr})
}

func (tc *TestCase) Run(t *testing.T) {
	span, ctx := tracer.StartSpanFromContext(context.Background(), "test.root")
	defer span.Finish()

	require.NoError(t, tc.Client.WithContext(ctx).Set(tc.key, "test_value", 0).Err())
	require.NoError(t, tc.Client.WithContext(ctx).Get(tc.key).Err())
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
						"component":         "go-redis/redis",
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
						"component":         "go-redis/redis",
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
