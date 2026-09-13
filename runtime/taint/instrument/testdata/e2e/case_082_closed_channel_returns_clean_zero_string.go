// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package main

import "os"

func init() {
	register(Case{
		ID:   82,
		Name: "closed channel returns clean zero string",
		Run: func() {
			_ = os.Setenv("CASE_082_INPUT", "secret")
			path := os.Getenv("CASE_082_INPUT")
			case082Channel := make(chan string)
			close(case082Channel)
			path = <-case082Channel
			_, _ = os.Open(path)
		},
		// Empirically confirmed via captured=[]: receiving from a closed,
		// empty channel overwrites path with the zero string "", which has
		// no registered backing-array taint, so no report fires. Want is
		// omitted (no report expected at all).
	})
}
