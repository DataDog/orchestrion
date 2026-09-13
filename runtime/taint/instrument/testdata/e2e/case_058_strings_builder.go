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
		ID:   58,
		Name: "strings builder",
		Run: func() {
			_ = os.Setenv("CASE_058_INPUT", "secret")
			source := os.Getenv("CASE_058_INPUT")
			var builder strings.Builder
			_, _ = builder.WriteString("builder-")
			_, _ = builder.WriteString(source)
			_, _ = os.Open(builder.String())
		},
		Want: []Expect{{Value: "builder-secret", Ranges: []taint.Range{{Start: 8, End: 14}}}},
	})
}
