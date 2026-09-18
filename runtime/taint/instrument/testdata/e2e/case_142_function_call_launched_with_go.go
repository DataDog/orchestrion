// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package main

import (
	"os"
	"sync"

	"github.com/DataDog/orchestrion/runtime/taint"
)

func init() {
	register(Case{
		ID:   142,
		Name: "function call launched with go",
		Run: func() {
			_ = os.Setenv("CASE_142_INPUT", "secret")
			var case142Wait sync.WaitGroup
			case142Wait.Add(1)
			// The argument is evaluated here, on the parent goroutine, at the
			// point the go statement executes; only the already-computed
			// string header is handed to the child goroutine's frame.
			go func(path string) {
				defer case142Wait.Done()
				_, _ = os.Open(path)
			}(os.Getenv("CASE_142_INPUT"))
			// Wait for the child goroutine's sink call to complete before Run
			// returns: the harness swaps its capturing reporter back out
			// immediately after Run, so an in-flight report would otherwise
			// be lost or misattributed to a later case.
			case142Wait.Wait()
		},
		// Empirically confirmed via a deliberately wrong probe value, which
		// surfaced captured=[{os.Open secret [{0 6}]}]: the string header
		// produced by os.Getenv keeps its backing-array address as it is
		// copied into the go statement's argument and then into the new
		// goroutine's stack frame, so the registry lookup inside os.Open
		// (running on the child goroutine) still finds the exact range.
		Want: []Expect{{Value: "secret", Ranges: []taint.Range{{Start: 0, End: 6}}}},
	})
}
