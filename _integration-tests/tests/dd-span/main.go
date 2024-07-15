// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package ddspan

import (
	"context"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
	"gopkg.in/DataDog/dd-trace-go.v1/ddtrace/tracer"
)

type TestCase struct{}

func (tc *TestCase) Setup(t *testing.T) {}

func (tc *TestCase) Run(t *testing.T) {
	span, ctx := tracer.StartSpanFromContext(context.Background(), "test.root")
	defer span.Finish()

	req, err := http.NewRequestWithContext(ctx, "GET", "http://localhost:0/", nil)
	require.NoError(t, err)

	spanFromHttpRequest(req)
}

func (tc *TestCase) Teardown(t *testing.T) {}

//dd:span foo:bar
func spanFromHttpRequest(req *http.Request) string {
	return tagSpecificSpan(req.Context())
}
