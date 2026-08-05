// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package main

import (
	"os"

	"github.com/DataDog/orchestrion/runtime/taint"
)

//go:noinline
func case008Recur(value string, depth int) string {
	if depth == 0 {
		return value
	}
	return case008Recur(value, depth-1)
}

func init() {
	register(Case{
		ID:   8,
		Name: "static recursion",
		Run: func() {
			_ = os.Setenv("CASE_008_INPUT", "secret")
			source := os.Getenv("CASE_008_INPUT")
			_, _ = os.Open(case008Recur(source, 200))
		},
		Want: []Expect{{Value: "secret", Ranges: []taint.Range{{Start: 0, End: 6}}}},
	})
}
