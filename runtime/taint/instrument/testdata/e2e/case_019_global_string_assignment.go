// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package main

import (
	"os"

	"github.com/DataDog/orchestrion/runtime/taint"
)

var case019Global string

func init() {
	register(Case{
		ID:   19,
		Name: "global string assignment",
		Run: func() {
			_ = os.Setenv("CASE_019_INPUT", "secret")
			case019Global = os.Getenv("CASE_019_INPUT")
			_, _ = os.Open(case019Global)
		},
		Want: []Expect{{Value: "secret", Ranges: []taint.Range{{Start: 0, End: 6}}}},
	})
}
