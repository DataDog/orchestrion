// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

//go:build integration

package k8sclientgo

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"

	"orchestrion/integration/validator/trace"
)

type TestCaseStructLiteralWithParam struct {
	base
	wtCalled bool
}

func (tc *TestCaseStructLiteralWithParam) Setup(t *testing.T) {
	tc.base.setup(t)

	cfg := &rest.Config{
		Host: tc.server.URL,
		WrapTransport: func(rt http.RoundTripper) http.RoundTripper {
			tc.wtCalled = true
			return rt
		},
	}

	client, err := kubernetes.NewForConfig(cfg)
	require.NoError(t, err)
	tc.base.client = client
}

func (tc *TestCaseStructLiteralWithParam) Run(t *testing.T) {
	tc.base.run(t)
	assert.True(t, tc.wtCalled, "the original WrapTransport function was not called")
}

func (tc *TestCaseStructLiteralWithParam) Teardown(t *testing.T) {
	tc.base.teardown(t)
}

func (tc *TestCaseStructLiteralWithParam) ExpectedTraces() trace.Spans {
	return tc.base.expectedSpans()
}
