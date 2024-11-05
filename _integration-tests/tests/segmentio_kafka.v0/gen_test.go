// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.
//
// Code generated by 'go generate'; DO NOT EDIT.

//go:build integration

package segmentio_kafka_v0

import (
	"datadoghq.dev/orchestrion/_integration-tests/utils"
	"testing"
)

func TestIntegration_segmentio_kafka_v0(t *testing.T) {
	utils.RunTest(t, new(TestCase))
}
