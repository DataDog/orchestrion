// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

//go:build integration

package awsv2

import (
	"fmt"
	"testing"

	"orchestrion/integration/validator/trace"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
)

type TestCaseNewConfig struct {
	base
}

func (tc *TestCaseNewConfig) Setup(t *testing.T) {
	tc.setup(t)

	cfg := aws.NewConfig()
	cfg.Region = "test-region-1337"
	cfg.Credentials = credentials.NewStaticCredentialsProvider("NOTANACCESSKEY", "NOTASECRETKEY", "")
	cfg.BaseEndpoint = aws.String(fmt.Sprintf("http://%s:%s", tc.host, tc.port))
	tc.cfg = *cfg
}

func (tc *TestCaseNewConfig) Run(t *testing.T) {
	tc.base.run(t)
}

func (tc *TestCaseNewConfig) Teardown(t *testing.T) {
	tc.base.teardown(t)
}

func (tc *TestCaseNewConfig) ExpectedTraces() trace.Spans {
	return tc.base.expectedSpans()
}
