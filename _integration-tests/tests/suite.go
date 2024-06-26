// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package tests

import (
	"testing"

	"orchestrion/integration/validator/trace"
)

//go:generate go run ../utils/generator

type testCase interface {
	Setup(*testing.T)
	Run(*testing.T)
	Teardown(*testing.T)

	ExpectedTraces() trace.Spans
}
