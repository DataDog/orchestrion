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
		ID:   41,
		Name: "strings repeat",
		Run: func() {
			_ = os.Setenv("CASE_041_INPUT", "secret")
			source := os.Getenv("CASE_041_INPUT")
			_, _ = os.Open("repeat-" + strings.Repeat(source, 2))
		},
		Want: []Expect{{Value: "repeat-secretsecret", Ranges: []taint.Range{{Start: 7, End: 19}}}},
	})
}
