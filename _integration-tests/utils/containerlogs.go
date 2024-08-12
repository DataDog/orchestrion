// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package utils

import (
	"testing"

	"github.com/testcontainers/testcontainers-go"
)

func WithTestLogConsumer(t *testing.T) testcontainers.CustomizeRequestOption {
	return testcontainers.WithLogConsumers(TestLogConsumer(t))
}

type testLogConsumer struct {
	*testing.T
}

func TestLogConsumer(t *testing.T) testcontainers.LogConsumer {
	return testLogConsumer{t}
}

func (t testLogConsumer) Accept(log testcontainers.Log) {
	t.T.Log(string(log.Content))
}
