// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package sarama

import (
	"orchestrion/integration/validator/trace"
	"testing"

	"github.com/Shopify/sarama"
	"github.com/testcontainers/testcontainers-go"
)

type TestCase struct {
	server testcontainers.Container
	cfg    *sarama.Config
}

func (tc *TestCase) Setup(t *testing.T) {

}

func (tc *TestCase) Run(t *testing.T) {

}

func (tc *TestCase) Teardown(t *testing.T) {

}

func (tc *TestCase) ExpectedTraces() trace.Spans {
	return trace.Spans{
		{},
	}
}
