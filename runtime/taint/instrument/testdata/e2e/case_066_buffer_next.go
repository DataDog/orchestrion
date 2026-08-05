// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package main

import (
	"bytes"
	"os"

	"github.com/DataDog/orchestrion/runtime/taint"
)

func init() {
	register(Case{
		ID:   66,
		Name: "buffer next",
		Run: func() {
			_ = os.Setenv("CASE_066_INPUT", "secret")
			source := os.Getenv("CASE_066_INPUT")
			var buffer bytes.Buffer
			_, _ = buffer.WriteString("x")
			_, _ = buffer.WriteString(source)
			buffer.Next(1)
			_, _ = os.Open("next-" + buffer.String())
		},
		Want: []Expect{{Value: "next-secret", Ranges: []taint.Range{{Start: 5, End: 11}}}},
	})
}
