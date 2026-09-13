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
		ID:   144,
		Name: "string slice element store and load",
		Run: func() {
			_ = os.Setenv("CASE_144_INPUT", "secret")
			case144Tainted := os.Getenv("CASE_144_INPUT")
			case144Values := []string{"clean-first", case144Tainted, "clean-last"}

			_, _ = os.Open(case144Values[0])
			_, _ = os.Open(case144Values[2])
			_, _ = os.Open(case144Values[1])
		},
		Want: []Expect{{Value: "secret", Ranges: []taint.Range{{Start: 0, End: 6}}}},
	})
}
