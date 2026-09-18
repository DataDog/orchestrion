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
		ID:   25,
		Name: "rune conversion",
		Run: func() {
			_ = os.Setenv("CASE_025_INPUT", "secret")
			source := os.Getenv("CASE_025_INPUT")
			value := []rune(source)
			_, _ = os.Open("rune-" + string(value))
		},
		Want: []Expect{{Value: "rune-secret", Ranges: []taint.Range{{Start: 5, End: 11}}}},
	})
}
