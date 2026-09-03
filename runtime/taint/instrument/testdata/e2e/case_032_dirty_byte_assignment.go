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
		ID:   32,
		Name: "dirty byte assignment",
		Run: func() {
			_ = os.Setenv("CASE_032_INPUT", "secret")
			source := os.Getenv("CASE_032_INPUT")
			indexed := []byte(source)
			assigned := []byte("x")
			assigned[0] = indexed[0]
			_, _ = os.Open("assigned-" + string(assigned))
		},
		Want: []Expect{{Value: "assigned-s", Ranges: []taint.Range{{Start: 9, End: 10}}}},
	})
}
