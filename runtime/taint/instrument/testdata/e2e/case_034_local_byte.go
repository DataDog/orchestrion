// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package main

import (
	"os"

	"github.com/DataDog/orchestrion/runtime/taint"
)

func init() {
	register(Case{
		ID:   34,
		Name: "local byte",
		Run: func() {
			_ = os.Setenv("CASE_034_INPUT", "secret")
			source := os.Getenv("CASE_034_INPUT")
			value := source[0]
			_, _ = os.Open("local-byte-" + string(value))
		},
		Want: []Expect{{Value: "local-byte-s", Ranges: []taint.Range{{Start: 11, End: 12}}}},
	})
}
