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
		ID:   126,
		Name: "multiple exact source root sets",
		Run: func() {
			cleanLeft := "clean-left"
			cleanRight := "clean-right"
			_, _ = os.Open(strings.ToUpper(cleanLeft + cleanRight))

			_ = os.Setenv("CASE_126_LEFT", "alpha")
			_ = os.Setenv("CASE_126_RIGHT", "beta")
			left := os.Getenv("CASE_126_LEFT")
			right := os.Getenv("CASE_126_RIGHT")
			_, _ = os.Open(strings.ToUpper(left + right))
		},
		Want: []Expect{{
			Value:             "ALPHABETA",
			Ranges:            []taint.Range{{Start: 0, End: 9}, {Start: 0, End: 9}},
			DistinctSourceIDs: 2,
		}},
	})
}
