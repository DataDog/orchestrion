// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

//go:build integration && windows

package kafka

import (
	"testing"

	"datadoghq.dev/orchestrion/_integration-tests/validator/trace"
)

type skip struct{}

func (skip) Setup(t *testing.T) {
	t.Skip("skipping test since confluent-kafka-go requires extra setup to build on Windows: https://github.com/confluentinc/confluent-kafka-go/issues/889")
}

func (skip) Run(t *testing.T)             {}
func (skip) ExpectedTraces() trace.Traces { return nil }

type TestCase = skip
