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

type TestCaseFuncHandler struct {
	base
}

func (tc *TestCaseFuncHandler) Setup(t *testing.T) {
	tc.handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/":
			tc.handleRoot(w, r)
			return

		case "/hit":
			tc.handleHit(w, r)
			return

		default:
			w.WriteHeader(http.StatusNotFound)
			return
		}
	})

	tc.base.Setup(t)
}
