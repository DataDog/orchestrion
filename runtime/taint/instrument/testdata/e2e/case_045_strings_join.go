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
		ID:   45,
		Name: "strings join",
		Run: func() {
			_ = os.Setenv("CASE_045_INPUT", "secret")
			source := os.Getenv("CASE_045_INPUT")
			_, _ = os.Open(strings.Join([]string{"join", source}, "-"))
		},
		Want: []Expect{{Value: "join-secret", Ranges: []taint.Range{{Start: 5, End: 11}}}},
	})
}
