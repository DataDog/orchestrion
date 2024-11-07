// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

//go:build integration

package awsv2

import (
	"fmt"
	"testing"

	"datadoghq.dev/orchestrion/_integration-tests/validator/trace"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
)

type TestCaseStructLiteralPtr struct {
	base
}

func (tc *TestCaseStructLiteralPtr) Setup(t *testing.T) {
	tc.setup(t)

	cfg := &aws.Config{
		Region:       "test-region-1337",
		Credentials:  credentials.NewStaticCredentialsProvider("NOTANACCESSKEY", "NOTASECRETKEY", ""),
		BaseEndpoint: aws.String(fmt.Sprintf("http://%s:%s", tc.host, tc.port)),
	}
	tc.cfg = *cfg
}

func (tc *TestCaseStructLiteralPtr) Run(t *testing.T) {
	tc.base.run(t)
}

func (tc *TestCaseStructLiteralPtr) ExpectedTraces() trace.Traces {
	return tc.base.expectedTraces()
}
