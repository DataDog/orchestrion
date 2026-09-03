// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package main

import (
	"os"
	"path"

	"github.com/DataDog/orchestrion/runtime/taint"
)

func init() {
	register(Case{
		ID:   156,
		Name: "path clean fresh string",
		Run: func() {
			_ = os.Setenv("CASE_156_INPUT", "secret")
			_, _ = os.Open(path.Clean("a/../case156-clean"))
			dirty := "a/../" + os.Getenv("CASE_156_INPUT")
			_, _ = os.Open(path.Clean(dirty))
		},
		Want: []Expect{{Value: "secret", Ranges: []taint.Range{{Start: 0, End: 6}}}},
	})
}
