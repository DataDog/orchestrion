// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

//go:build integration

package redigo

import (
	"context"
	"fmt"
	"log"
	"net/url"
	"testing"
	"time"

	"datadoghq.dev/orchestrion/_integration-tests/utils"
	"datadoghq.dev/orchestrion/_integration-tests/validator/trace"
	"github.com/gomodule/redigo/redis"
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
	*redis.Pool
	key string
}

func (tc *TestCase) Setup(t *testing.T) {
	ctx := context.Background()

	uuid, err := uuid.NewRandom()
	require.NoError(t, err)
	tc.key = uuid.String()

	const network = "tcp"
	addr := "localhost:6379"
	if !utils.IsGithubActions {
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
		utils.AssertTestContainersError(t, err)

		redisURI, err := tc.server.ConnectionString(ctx)
		if err != nil {
			log.Fatalf("Failed to obtain connection string: %v\n", err)
		}
		redisURL, err := url.Parse(redisURI)
		if err != nil {
			log.Fatalf("Invalid redis connection string: %q\n", redisURI)
		}

		addr = redisURL.Host
	}

	var dialOptions = []redis.DialOption{
		redis.DialReadTimeout(10 * time.Second),
	}

	tc.Pool = &redis.Pool{
		Dial: func() (redis.Conn, error) { return redis.Dial(network, addr, dialOptions...) },
		DialContext: func(ctx context.Context) (redis.Conn, error) {
			return redis.DialContext(ctx, network, addr)
		},
		TestOnBorrow: func(c redis.Conn, _ time.Time) error {
			_, err := c.Do("PING")
			return err
		},
	}

	client := tc.Pool.Get()
	defer func() { require.NoError(t, client.Close()) }()
	_, err = client.Do("SET", tc.key, "test_value")
	require.NoError(t, err)
}

func (tc *TestCase) Run(t *testing.T) {
	span, ctx := tracer.StartSpanFromContext(context.Background(), "test.root")
	defer span.Finish()

	client, err := tc.Pool.GetContext(ctx)
	require.NoError(t, err)
	defer func() { require.NoError(t, client.Close()) }()

	res, err := client.Do("GET", tc.key, ctx)
	require.NoError(t, err)
	require.NotEmpty(t, res)
}

func (tc *TestCase) Teardown(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	assert.NoError(t, tc.Pool.Close())
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
						"resource": "GET",
						"type":     "redis",
						"name":     "redis.command",
						"service":  "redis.conn",
					},
					Meta: map[string]string{
						"redis.raw_command": fmt.Sprintf("GET %s", tc.key),
						"db.system":         "redis",
						"component":         "gomodule/redigo",
						"out.network":       "tcp",
						"out.host":          "localhost",
						"redis.args_length": "1",
						"span.kind":         "client",
					},
				},
				{
					Tags: map[string]any{
						"resource": "redigo.Conn.Flush",
						"type":     "redis",
						"name":     "redis.command",
						"service":  "redis.conn",
					},
					Meta: map[string]string{
						"redis.raw_command": "",
						"db.system":         "redis",
						"component":         "gomodule/redigo",
						"out.network":       "tcp",
						"out.host":          "localhost",
						"redis.args_length": "0",
						"span.kind":         "client",
					},
				},
			},
		},
	}
}
