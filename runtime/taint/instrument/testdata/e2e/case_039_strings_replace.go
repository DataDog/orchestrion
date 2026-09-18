// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package main

import (
	"os"
	"strings"

	"github.com/DataDog/orchestrion/runtime/taint"
)

func init() {
	register(Case{
		ID:   39,
		Name: "strings replace",
		Run: func() {
			_ = os.Setenv("CASE_039_INPUT", "secret")
			source := os.Getenv("CASE_039_INPUT")
			_, _ = os.Open(strings.Replace("replace-value", "value", source, 1))
		},
		Want: []Expect{{Value: "replace-secret", Ranges: []taint.Range{{Start: 8, End: 14}}}},
	})
}
