// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.
//
// Code generated by 'go generate'; DO NOT EDIT.

//go:build integration

package awsv2

import (
	utils "orchestrion/integration/utils"
	"testing"
)

func TestIntegration(t *testing.T) {
	testCases := map[string]utils.TestCase{
		"LoadDefaultConfig": new(TestCaseLoadDefaultConfig),
		"NewConfig":         new(TestCaseNewConfig),
		"StructLiteral":     new(TestCaseStructLiteral),
		"StructLiteralPtr":  new(TestCaseStructLiteralPtr),
	}
	runTest := utils.NewTestSuite(testCases)
	runTest(t)
}
