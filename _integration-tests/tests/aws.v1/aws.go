// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

//go:build integration

package awsv1

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"

	"orchestrion/integration/utils"
	"orchestrion/integration/validator/trace"
)

type TestCase struct {
	server testcontainers.Container
	cfg    *aws.Config
}

func (tc *TestCase) Setup(t *testing.T) {
	server, host, port := utils.StartDynamoDBTestContainer(t)
	tc.server = server

	tc.cfg = &aws.Config{
		Credentials: credentials.NewStaticCredentials("NOTANACCESSKEY", "NOTASECRETKEY", ""),
		Endpoint:    aws.String(fmt.Sprintf("http://%s:%s", host, port)),
		Region:      aws.String("test-region-1337"),
	}
}

func (tc *TestCase) Run(t *testing.T) {
	ddb := dynamodb.New(session.Must(session.NewSession(tc.cfg)))
	_, err := ddb.ListTables(nil)
	require.NoError(t, err)
}

func (tc *TestCase) Teardown(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	require.NoError(t, tc.server.Terminate(ctx))
}

func (*TestCase) ExpectedTraces() trace.Spans {
	return trace.Spans{
		{
			Tags: map[string]any{
				"name":     "dynamodb.command",
				"service":  "aws.dynamodb",
				"resource": "dynamodb.ListTables",
				"type":     "http",
			},
			Meta: map[string]any{
				"aws.operation":    "ListTables",
				"aws.region":       "test-region-1337",
				"aws_service":      "dynamodb",
				"http.method":      "POST",
				"http.status_code": "200",
				"component":        "aws/aws-sdk-go/aws",
				"span.kind":        "client",
			},
		},
	}
}
