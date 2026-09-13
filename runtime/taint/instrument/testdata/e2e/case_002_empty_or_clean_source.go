// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package main

import "os"

func init() {
	register(Case{
		ID:   2,
		Name: "empty or clean source",
		Run: func() {
			// Half 1: an empty environment value flowing straight to the sink.
			_ = os.Setenv("CASE_002_EMPTY_INPUT", "")
			case002EmptySource := os.Getenv("CASE_002_EMPTY_INPUT")
			_, _ = os.Open(case002EmptySource)

			// Half 2: a non-empty, non-source-derived literal flowing to the
			// sink. It never passes through os.Getenv or any other rewritten
			// call, so it carries no taint to lose or keep.
			case002CleanLiteral := "case002-clean-marker"
			_, _ = os.Open(case002CleanLiteral)
		},
		// Empirically confirmed via captured=[]: neither half produces a
		// report, so Want is omitted (no report expected at all).
	})
}
