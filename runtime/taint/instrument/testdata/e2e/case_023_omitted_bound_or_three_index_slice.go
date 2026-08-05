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
		ID:   23,
		Name: "omitted-bound or three-index slice",
		Run: func() {
			_ = os.Setenv("CASE_023_INPUT", "secretXYZ")
			source := os.Getenv("CASE_023_INPUT")
			data := []byte(source)
			window := data[:3:3]
			_, _ = os.Open("win-" + string(window))
		},
		Want: []Expect{{Value: "win-sec", Ranges: []taint.Range{{Start: 4, End: 7}}}},
	})
}
