// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package main

import (
	"os"

	"github.com/DataDog/orchestrion/runtime/taint"
)

//go:noinline
func case103DirtyFrame() {
	_, _ = os.Open(os.Getenv("CASE_103_INPUT"))
}

//go:noinline
func case103CleanFrame() {
	_, _ = os.Open("clean")
}

func init() {
	register(Case{
		ID:   103,
		Name: "stack reuse clears old labels",
		Run: func() {
			_ = os.Setenv("CASE_103_INPUT", "secret")
			case103DirtyFrame()
			case103CleanFrame()
		},
		// Empirically confirmed via captured=[{os.Open secret [{0 6}]}]: the
		// dirty frame's os.Getenv->os.Open pass-through reports "secret" with
		// its full byte range, and case103CleanFrame's reused stack slots do
		// NOT inherit that taint — the "clean" literal sink produces no
		// second report.
		Want: []Expect{{Value: "secret", Ranges: []taint.Range{{Start: 0, End: 6}}}},
	})
}
