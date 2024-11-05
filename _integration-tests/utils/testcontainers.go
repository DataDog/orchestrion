// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package utils

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"runtime"
	"testing"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/go-connections/nat"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/kafka"
	"github.com/testcontainers/testcontainers-go/modules/redis"
	"github.com/testcontainers/testcontainers-go/wait"
)

// StartDynamoDBTestContainer starts a new dynamoDB test container and returns the necessary information to connect
// to it.
func StartDynamoDBTestContainer(t *testing.T) (testcontainers.Container, string, string) {
	exposedPort := "8000/tcp"
	req := testcontainers.GenericContainerRequest{
		ContainerRequest: testcontainers.ContainerRequest{
			Image:        "amazon/dynamodb-local:latest",
			ExposedPorts: []string{exposedPort},
			WaitingFor:   wait.ForHTTP("").WithStatusCodeMatcher(func(int) bool { return true }),
			WorkingDir:   "/home/dynamodblocal",
			Cmd: []string{
				"-jar", "DynamoDBLocal.jar",
				"-inMemory",
				"-disableTelemetry",
			},
			LogConsumerCfg: &testcontainers.LogConsumerConfig{
				Consumers: []testcontainers.LogConsumer{TestLogConsumer(t)},
			},
		},
		Started: true,
		Logger:  testcontainers.TestLogger(t),
	}

	ctx := context.Background()
	server, err := testcontainers.GenericContainer(ctx, req)
	AssertTestContainersError(t, err)

	mappedPort, err := server.MappedPort(ctx, nat.Port(exposedPort))
	require.NoError(t, err)

	host, err := server.Host(ctx)
	require.NoError(t, err)

	return server, host, mappedPort.Port()
}

// StartKafkaTestContainer starts a new Kafka test container and returns the connection string.
func StartKafkaTestContainer(t *testing.T) (*kafka.KafkaContainer, string) {
	ctx := context.Background()
	exposedPort := "9093/tcp"

	container, err := kafka.Run(ctx,
		"confluentinc/confluent-local:7.5.0",
		kafka.WithClusterID("test-cluster"),
		WithTestLogConsumer(t),
	)
	AssertTestContainersError(t, err)

	mappedPort, err := container.MappedPort(ctx, nat.Port(exposedPort))
	require.NoError(t, err)

	host, err := container.Host(ctx)
	require.NoError(t, err)

	addr := fmt.Sprintf("%s:%s", host, mappedPort.Port())
	return container, addr
}

// StartRedisTestContainer starts a new Redis test container and returns the connection string.
func StartRedisTestContainer(t *testing.T) (*redis.RedisContainer, string) {
	ctx := context.Background()
	exposedPort := "6379/tcp"

	container, err := redis.Run(ctx,
		"redis:7",
		testcontainers.WithLogger(testcontainers.TestLogger(t)),
		WithTestLogConsumer(t),
		testcontainers.WithWaitStrategy(
			wait.ForAll(
				wait.ForLog("* Ready to accept connections"),
				wait.ForExposedPort(),
				wait.ForListeningPort(nat.Port(exposedPort)),
			),
		),
		testcontainers.WithHostConfigModifier(func(hostConfig *container.HostConfig) {
			if hostConfig.Sysctls == nil {
				hostConfig.Sysctls = make(map[string]string)
			}
			hostConfig.Sysctls["vm.overcommit_memory"] = "1"
		}),
	)
	AssertTestContainersError(t, err)

	connStr, err := container.ConnectionString(ctx)
	require.NoError(t, err)

	redisURL, err := url.Parse(connStr)
	require.NoError(t, err)

	return container, redisURL.Host
}

// AssertTestContainersError decides whether the provided testcontainers error should make the test fail or mark it as
// skipped, depending on the environment where the test is running.
func AssertTestContainersError(t *testing.T, err error) {
	if err == nil {
		return
	}
	if _, ok := os.LookupEnv("CI"); ok && runtime.GOOS != "linux" {
		t.Skipf("failed to start container (CI does not support docker, skipping test): %v", err)
		return
	}
	require.NoError(t, err)
}

// SkipIfProviderIsNotHealthy calls [testcontainers.SkipIfProviderIsNotHealthy] to skip tests of
// the testcontainers provider is not healthy or running at all; except when the test is running in
// CI mode (the CI environment variable is defined) and the GOOS is linux.
func SkipIfProviderIsNotHealthy(t *testing.T) {
	t.Helper()

	if _, ci := os.LookupEnv("CI"); ci && runtime.GOOS == "linux" {
		// We never want to skip tests on Linux CI, as this could lead to not noticing the tests are not
		// running at all, resulting in usurped confidence in the (un)tested code.
		return
	}

	defer func() {
		err := recover()
		if err == nil {
			return
		}
		// We recovered from a panic (e.g, "rootless Docker not found" on GitHub Actions + macOS), so we
		// will behave as if the provider was not healthy (because it's not and shouldn't have panic'd
		// in the first place).
		t.Log(err)
		t.SkipNow()
	}()

	testcontainers.SkipIfProviderIsNotHealthy(t)
}
