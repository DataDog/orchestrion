// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

//go:build integration

package awsv2

import (
	"context"
	"fmt"
	"os"
	"runtime"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/docker/go-connections/nat"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"

	"orchestrion/integration/utils"
	"orchestrion/integration/validator/trace"
)

type base struct {
	server     testcontainers.Container
	cfg        aws.Config
	hostIP     string
	mappedPort string
}

func (b *base) setup(t *testing.T) {
	if _, ok := os.LookupEnv("CI"); ok && runtime.GOOS != "linux" {
		t.Skip("skipping test as it requires docker to run in the CI")
	}

	port := "8000/tcp"
	req := testcontainers.GenericContainerRequest{
		ContainerRequest: testcontainers.ContainerRequest{
			Image:        "amazon/dynamodb-local:latest",
			ExposedPorts: []string{port},
			WaitingFor:   wait.ForHTTP("").WithStatusCodeMatcher(func(int) bool { return true }),
			Name:         "dynamodb-local",
			WorkingDir:   "/home/dynamodblocal",
			Cmd: []string{
				"-jar", "DynamoDBLocal.jar",
				"-inMemory",
				"-disableTelemetry",
			},
			LogConsumerCfg: &testcontainers.LogConsumerConfig{
				Consumers: []testcontainers.LogConsumer{utils.TestLogConsumer(t)},
			},
		},
		Started: true,
		Logger:  testcontainers.TestLogger(t),
	}

	ctx := context.Background()
	var err error
	b.server, err = testcontainers.GenericContainer(ctx, req)
	require.NoError(t, err)

	mappedPort, err := b.server.MappedPort(ctx, nat.Port(port))
	require.NoError(t, err)
	b.mappedPort = mappedPort.Port()

	b.hostIP, err = b.server.Host(ctx)
	require.NoError(t, err)
}

func (b *base) teardown(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	require.NoError(t, b.server.Terminate(ctx))
}

func (b *base) run(t *testing.T) {
	if len(b.cfg.APIOptions) == 0 {
		t.Log("the AWS config is not traced")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()
	ddb := dynamodb.NewFromConfig(b.cfg)
	_, err := ddb.ListTables(ctx, nil)
	require.NoError(t, err)
}

func (b *base) expectedSpans() trace.Spans {
	return trace.Spans{
		{
			Tags: map[string]any{
				"name":     "DynamoDB.request",
				"service":  "aws.DynamoDB",
				"resource": "DynamoDB.ListTables",
				"type":     "http",
			},
			Meta: map[string]any{
				"aws.operation": "ListTables",
				"aws.region":    "test-region-1337",
				"aws_service":   "DynamoDB",
				"http.method":   "POST",
				// TODO: investigate why this tag is not being set
				// "http.status_code": "200",
				"component": "aws/aws-sdk-go-v2/aws",
				"span.kind": "client",
			},
			Children: []*trace.Span{
				{
					Tags: map[string]any{
						"name":     "http.request",
						"service":  "aws.DynamoDB",
						"resource": "POST /",
						"type":     "http",
					},
					Meta: map[string]any{
						"http.method":              "POST",
						"http.status_code":         "200",
						"http.url":                 fmt.Sprintf("http://localhost:%s/", b.mappedPort),
						"network.destination.name": "localhost",
						"component":                "net/http",
						"span.kind":                "client",
					},
				},
			},
		},
	}
}
