// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

//go:build integration

package redigo

import (
	"context"
	"log"
	"net/url"
	"orchestrion/integration/utils"
	"orchestrion/integration/validator/trace"
	"testing"
	"time"

	"github.com/gomodule/redigo/redis"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	testredis "github.com/testcontainers/testcontainers-go/modules/redis"
	"github.com/testcontainers/testcontainers-go/wait"
	"gopkg.in/DataDog/dd-trace-go.v1/ddtrace/tracer"
)

type TestCase struct {
	server *testredis.RedisContainer
	*redis.Pool
}

func (tc *TestCase) Setup(t *testing.T) {
	ctx := context.Background()

	var err error
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
	if err != nil {
		t.Skipf("Failed to start redis test container: %v\n", err)
	}
	redisURI, err := tc.server.ConnectionString(ctx)
	if err != nil {
		log.Fatalf("Failed to obtain connection string: %v\n", err)
	}
	redisURL, err := url.Parse(redisURI)
	if err != nil {
		log.Fatalf("Invalid redis connection string: %q\n", redisURI)
	}

	const network = "tcp"
	address := redisURL.Host

	tc.Pool = &redis.Pool{
		Dial:        func() (redis.Conn, error) { return redis.Dial(network, address) },
		DialContext: func(ctx context.Context) (redis.Conn, error) { return redis.DialContext(ctx, network, address) },
		TestOnBorrow: func(c redis.Conn, _ time.Time) error {
			_, err := c.Do("PING")
			return err
		},
	}

	client := tc.Pool.Get()
	defer func() { require.NoError(t, client.Close()) }()
	_, err = client.Do("SET", "test_key", "test_value")
	require.NoError(t, err)
}

func (tc *TestCase) Run(t *testing.T) {
	span, ctx := tracer.StartSpanFromContext(context.Background(), "test.root")
	defer span.Finish()

	client, err := tc.Pool.GetContext(ctx)
	require.NoError(t, err)
	defer func() { require.NoError(t, client.Close()) }()

	res, err := client.Do("GET", "test_key", ctx)
	require.NoError(t, err)
	require.NotEmpty(t, res)
}

func (tc *TestCase) Teardown(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	assert.NoError(t, tc.Pool.Close())
	assert.NoError(t, tc.server.Terminate(ctx))
}

func (tc *TestCase) ExpectedTraces() trace.Spans {
	return trace.Spans{
		{
			Tags: map[string]any{
				"name": "test.root",
			},
			Children: trace.Spans{
				{
					Tags: map[string]any{
						"resource": "GET",
						"type":     "redis",
						"name":     "redis.command",
						"service":  "redis.conn",
					},
					Meta: map[string]any{
						"redis.raw_command": "GET test_key",
						"db.system":         "redis",
						"component":         "gomodule/redigo",
						"out.network":       "tcp",
						"out.host":          "localhost",
						"redis.args_length": "1",
						"span.kind":         "client",
					},
				},
			},
		},
		{
			Tags: map[string]any{
				"resource": "redigo.Conn.Flush",
				"type":     "redis",
				"name":     "redis.command",
				"service":  "redis.conn",
			},
			Meta: map[string]any{
				"redis.raw_command": "",
				"db.system":         "redis",
				"component":         "gomodule/redigo",
				"out.network":       "tcp",
				"out.host":          "localhost",
				"redis.args_length": "0",
				"span.kind":         "client",
			},
		},
	}
}
