// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package main

import "os"

//go:noinline
func case014Ignore(string) string { return "clean" }

func init() {
	register(Case{
		ID:   14,
		Name: "tainted parameter ignored, clean return",
		Run: func() {
			_ = os.Setenv("CASE_014_INPUT", "secret")
			_, _ = os.Open(case014Ignore(os.Getenv("CASE_014_INPUT")))
		},
		// Empirically confirmed via captured=[]: the tainted argument is
		// discarded and the fresh "clean" literal carries no registered
		// backing-array taint, so no report fires. Want is omitted (no
		// report expected at all).
	})
}
