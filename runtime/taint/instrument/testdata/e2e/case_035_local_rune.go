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
		ID:   35,
		Name: "local rune",
		Run: func() {
			_ = os.Setenv("CASE_035_INPUT", "secret")
			source := os.Getenv("CASE_035_INPUT")
			value := []rune(source)[0]
			_, _ = os.Open("local-rune-" + string(value))
		},
		Want: []Expect{{Value: "local-rune-s", Ranges: []taint.Range{{Start: 11, End: 12}}}},
	})
}
