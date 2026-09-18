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
		ID:   157,
		Name: "two independently tainted inputs concatenated",
		Run: func() {
			cleanLeft := "clean-left"
			cleanRight := "clean-right"
			_, _ = os.Open(cleanLeft + cleanRight)

			_ = os.Setenv("CASE_157_LEFT", "alpha")
			_ = os.Setenv("CASE_157_RIGHT", "beta")
			left := os.Getenv("CASE_157_LEFT")
			right := os.Getenv("CASE_157_RIGHT")
			_, _ = os.Open(left + right)
		},
		Want: []Expect{{
			Value:             "alphabeta",
			Ranges:            []taint.Range{{Start: 0, End: 5}, {Start: 5, End: 9}},
			DistinctSourceIDs: 2,
		}},
	})
}
