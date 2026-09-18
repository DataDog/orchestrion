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
		ID:   40,
		Name: "strings replace all",
		Run: func() {
			_ = os.Setenv("CASE_040_INPUT", "secret")
			source := os.Getenv("CASE_040_INPUT")
			_, _ = os.Open("replace-all-" + strings.ReplaceAll(source, "e", "E"))
		},
		Want: []Expect{{Value: "replace-all-sEcrEt", Ranges: []taint.Range{{Start: 12, End: 13}, {Start: 14, End: 16}, {Start: 17, End: 18}}}},
	})
}
