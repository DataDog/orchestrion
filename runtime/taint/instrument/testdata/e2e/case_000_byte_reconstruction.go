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
		ID:   0,
		Name: "byte reconstruction",
		Run: func() {
			_ = os.Setenv("CASE_000_BYTE_INPUT", "secret")
			source := os.Getenv("CASE_000_BYTE_INPUT")
			value := []byte(source)
			_, _ = os.Open("byte-" + string(value[0]))
		},
		Want: []Expect{{Value: "byte-s", Ranges: []taint.Range{{Start: 5, End: 6}}}},
	})
}
