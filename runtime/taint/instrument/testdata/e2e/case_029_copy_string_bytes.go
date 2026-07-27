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
		ID:   29,
		Name: "copy string bytes",
		Run: func() {
			_ = os.Setenv("CASE_029_INPUT", "secret")
			source := os.Getenv("CASE_029_INPUT")
			copied := make([]byte, len(source))
			copy(copied, source)
			_, _ = os.Open("copy-" + string(copied))
		},
		Want: []Expect{{Value: "copy-secret", Ranges: []taint.Range{{Start: 5, End: 11}}}},
	})
}
