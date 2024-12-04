// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

//go:build integration

package nethttp

import (
	"net/http"
	"testing"
)

type TestCaseServeMuxHandler struct {
	base
}

func (tc *TestCaseServeMuxHandler) Setup(t *testing.T) {
	tc.handler = tc.serveMuxHandler()
	tc.base.Setup(t)
}

type TestCaseHandlerIsNil struct {
	base
}

func (tc *TestCaseHandlerIsNil) Setup(t *testing.T) {
	http.HandleFunc("/hit", tc.base.handleHit)
	http.HandleFunc("/", tc.base.handleRoot)
	tc.base.handler = nil // Set handler to nil to test http.DefaultServeMux
	tc.base.Setup(t)
}
