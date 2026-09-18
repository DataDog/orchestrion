// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package main

import (
	"os"

	"github.com/DataDog/orchestrion/runtime/taint"
)

func init() {
	register(Case{
		ID:   83,
		Name: "standalone buffered receive in select",
		Run: func() {
			_ = os.Setenv("CASE_083_INPUT", "secret")
			ch := make(chan string, 1)
			ch <- os.Getenv("CASE_083_INPUT")
			select {
			case path := <-ch:
				_, _ = os.Open(path)
			}
		},
		// Empirically confirmed via captured=[{os.Open secret [{0 6}]}]: the
		// single-arm select desugars to a plain receive at the SSA level, so
		// the tainted backing array copied into the buffered channel survives
		// the := binding inside the case clause and the sink call within it.
		Want: []Expect{{Value: "secret", Ranges: []taint.Range{{Start: 0, End: 6}}}},
	})
}
