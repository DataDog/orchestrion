// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

//go:build integration

package awsv2

import (
	"context"
	"fmt"
	"testing"

	"datadoghq.dev/orchestrion/_integration-tests/validator/trace"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/stretchr/testify/require"
)

type TestCaseLoadDefaultConfig struct {
	base
}

func (tc *TestCaseLoadDefaultConfig) Setup(t *testing.T) {
	tc.setup(t)

	cfg, err := config.LoadDefaultConfig(context.Background(),
		config.WithRegion("test-region-1337"),
		config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider("NOTANACCESSKEY", "NOTASECRETKEY", "")),
	)
	require.NoError(t, err)
	cfg.BaseEndpoint = aws.String(fmt.Sprintf("http://%s:%s", tc.host, tc.port))
	tc.cfg = cfg
}

func (tc *TestCaseLoadDefaultConfig) Run(t *testing.T) {
	tc.base.run(t)
}

func (tc *TestCaseLoadDefaultConfig) Teardown(t *testing.T) {
	tc.base.teardown(t)
}

func (tc *TestCaseLoadDefaultConfig) ExpectedTraces() trace.Traces {
	return tc.base.expectedTraces()
}
