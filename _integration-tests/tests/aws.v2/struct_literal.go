// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

//go:build integration

package awsv2

import (
	"fmt"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"

	"orchestrion/integration/validator/trace"
)

type TestCaseStructLiteral struct {
	base
}

func (tc *TestCaseStructLiteral) Setup(t *testing.T) {
	tc.setup(t)

	tc.cfg = aws.Config{
		Region:       "test-region-1337",
		Credentials:  credentials.NewStaticCredentialsProvider("NOTANACCESSKEY", "NOTASECRETKEY", ""),
		BaseEndpoint: aws.String(fmt.Sprintf("http://%s:%s", tc.hostIP, tc.mappedPort)),
	}
}

func (tc *TestCaseStructLiteral) Run(t *testing.T) {
	tc.base.run(t)
}

func (tc *TestCaseStructLiteral) Teardown(t *testing.T) {
	tc.base.teardown(t)
}

func (tc *TestCaseStructLiteral) ExpectedTraces() trace.Spans {
	return tc.base.expectedSpans()
}
