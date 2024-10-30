// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package utils

import (
	"context"
	"fmt"
	"os"
	"runtime"
	"testing"

	"github.com/docker/go-connections/nat"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/kafka"
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

func StartKafkaTestContainer(t *testing.T) (*kafka.KafkaContainer, string) {
	ctx := context.Background()
	exposedPort := "9093/tcp"

	container, err := kafka.Run(ctx,
		"confluentinc/confluent-local:7.5.0",
		kafka.WithClusterID("test-cluster"),
	)
	AssertTestContainersError(t, err)

	mappedPort, err := container.MappedPort(ctx, nat.Port(exposedPort))
	require.NoError(t, err)

	host, err := container.Host(ctx)
	require.NoError(t, err)

	addr := fmt.Sprintf("%s:%s", host, mappedPort.Port())
	return container, addr
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

	testcontainers.SkipIfProviderIsNotHealthy(t)
}
