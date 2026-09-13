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
		ID:   80,
		Name: "select blocking receive",
		Run: func() {
			_ = os.Setenv("CASE_080_INPUT", "secret")
			source := os.Getenv("CASE_080_INPUT")

			values := make(chan string, 1)
			values <- source
			blocked := make(chan string)

			var path string
			select {
			case path = <-values:
			case path = <-blocked:
			}

			_, _ = os.Open(path)
		},
		Want: []Expect{{Value: "secret", Ranges: []taint.Range{{Start: 0, End: 6}}}},
	})
}
