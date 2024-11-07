// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

//go:build integration && windows

package go_elasticsearch

import (
	"testing"

	"datadoghq.dev/orchestrion/_integration-tests/validator/trace"
)

type skip struct{}

func (skip) Setup(t *testing.T) {
	t.Skip("skipping test since go-elasticsearch v7 and v8 does not build on Windows with Orchestrion: https://github.com/golang/go/issues/70046")
}

func (skip) Run(t *testing.T)             {}
func (skip) ExpectedTraces() trace.Traces { return nil }

type TestCaseV6 = skip
type TestCaseV7 = skip
type TestCaseV8 = skip
