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
		ID:   9,
		Name: "dynamic recursion and stack growth",
		Run: func() {
			_ = os.Setenv("CASE_009_INPUT", "secret")
			var recur func(string, int) string
			recur = func(value string, depth int) string {
				if depth == 0 {
					return value
				}
				return recur(value, depth-1)
			}
			_, _ = os.Open(recur(os.Getenv("CASE_009_INPUT"), 10_000))
		},
		Want: []Expect{{Value: "secret", Ranges: []taint.Range{{Start: 0, End: 6}}}},
	})
}
