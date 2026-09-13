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
		ID:   26,
		Name: "append scalar byte",
		Run: func() {
			_ = os.Setenv("CASE_026_INPUT", "secret")
			source := os.Getenv("CASE_026_INPUT")
			value := append([]byte(source), '!')
			_, _ = os.Open(string(value))
		},
		Want: []Expect{{Value: "secret!", Ranges: []taint.Range{{Start: 0, End: 6}}}},
	})
}
