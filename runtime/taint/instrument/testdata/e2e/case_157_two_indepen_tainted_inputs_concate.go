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
			_ = os.Setenv("CASE_157_LEFT", "alpha")
			_ = os.Setenv("CASE_157_RIGHT", "beta")
			left := os.Getenv("CASE_157_LEFT")
			right := os.Getenv("CASE_157_RIGHT")
			_, _ = os.Open(left + right)
		},
		// Both LEFT ("alpha", 5 bytes) and RIGHT ("beta", 4 bytes) are
		// independently tainted, but the two adjacent ranges touch at byte 5
		// (left's End == right's Start) and get merged into one contiguous
		// range by normalizeRanges, which merges when Start <= previous End.
		// Empirically confirmed: captured=[{os.Open alphabeta [{0 9}]}].
		Want: []Expect{{Value: "alphabeta", Ranges: []taint.Range{{Start: 0, End: 9}}}},
	})
}
