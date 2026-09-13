// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package main

import (
	"os"
	"strings"

	"github.com/DataDog/orchestrion/runtime/taint"
)

func init() {
	register(Case{
		ID:   59,
		Name: "strings builder write",
		Run: func() {
			_ = os.Setenv("CASE_059_INPUT", "secret")
			source := os.Getenv("CASE_059_INPUT")
			value := []byte(source)
			var builder strings.Builder
			_, _ = builder.WriteString("builder-")
			_, _ = builder.Write(value)
			_, _ = os.Open(builder.String())
		},
		Want: []Expect{{Value: "builder-secret", Ranges: []taint.Range{{Start: 8, End: 14}}}},
	})
}
