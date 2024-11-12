// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

//go:build integration

package k8sclientgo

import (
	"testing"

	"datadoghq.dev/orchestrion/_integration-tests/validator/trace"
	"github.com/stretchr/testify/require"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

type TestCaseStructLiteralWithoutParam struct {
	base
	wtCalled bool
}

func (tc *TestCaseStructLiteralWithoutParam) Setup(t *testing.T) {
	tc.base.setup(t)

	cfg := &rest.Config{
		Host: tc.server.URL,
	}

	client, err := kubernetes.NewForConfig(cfg)
	require.NoError(t, err)
	tc.base.client = client
}

func (tc *TestCaseStructLiteralWithoutParam) Run(t *testing.T) {
	tc.base.run(t)
}

func (tc *TestCaseStructLiteralWithoutParam) ExpectedTraces() trace.Traces {
	return tc.base.expectedTraces()
}
