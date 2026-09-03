// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package main

import (
	"os"
	"strconv"

	"github.com/DataDog/orchestrion/runtime/taint"
)

func init() {
	register(Case{
		ID:   49,
		Name: "strconv quote fresh output",
		Run: func() {
			_ = os.Setenv("CASE_049_INPUT", "secret")
			value := strconv.Quote(os.Getenv("CASE_049_INPUT"))
			_, _ = os.Open(value)
		},
		Want: []Expect{{Value: "\"secret\"", Ranges: []taint.Range{{Start: 0, End: 8}}}},
	})
}
