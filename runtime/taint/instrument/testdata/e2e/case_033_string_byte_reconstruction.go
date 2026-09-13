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
		ID:   33,
		Name: "string byte reconstruction",
		Run: func() {
			_ = os.Setenv("CASE_033_INPUT", "secret")
			source := os.Getenv("CASE_033_INPUT")
			_, _ = os.Open("scalar-" + string(source[0]))
		},
		Want: []Expect{{Value: "scalar-s", Ranges: []taint.Range{{Start: 7, End: 8}}}},
	})
}
