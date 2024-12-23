// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

//go:build integration && (linux || !githubci)

package awsv2

import (
	"context"
	"fmt"
	"testing"

	"datadoghq.dev/orchestrion/_integration-tests/validator/trace"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
)

type TestCaseStructLiteralPtr struct {
	base
}

func (tc *TestCaseStructLiteralPtr) Setup(t *testing.T, ctx context.Context) {
	tc.setup(t, ctx)

	cfg := &aws.Config{
		Region:       "test-region-1337",
		Credentials:  credentials.NewStaticCredentialsProvider("NOTANACCESSKEY", "NOTASECRETKEY", ""),
		BaseEndpoint: aws.String(fmt.Sprintf("http://%s:%s", tc.host, tc.port)),
	}
	tc.cfg = *cfg
}

func (tc *TestCaseStructLiteralPtr) Run(t *testing.T, ctx context.Context) {
	tc.base.run(t, ctx)
}

func (tc *TestCaseStructLiteralPtr) ExpectedTraces() trace.Traces {
	return tc.base.expectedTraces()
}
