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
		ID:   37,
		Name: "strings clone",
		Run: func() {
			_ = os.Setenv("CASE_037_INPUT", "secret")
			source := os.Getenv("CASE_037_INPUT")
			_, _ = os.Open("clone-" + strings.Clone(source))
		},
		Want: []Expect{{Value: "clone-secret", Ranges: []taint.Range{{Start: 6, End: 12}}}},
	})
}
