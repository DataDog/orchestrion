// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package main

import (
	"os"
	"runtime"
	"unsafe"
	"weak"

	"github.com/DataDog/orchestrion/runtime/taint"
)

// case104Secret is kept well above the runtime's tiny-object batching
// threshold (16 bytes) so the weak pointer below tracks exactly one
// allocation instead of a shared tiny-allocator slot.
const case104Secret = "gc-sweep-heap-shadow-secret-payload"

// case104TaintedBox taints a fresh backing array through os.Getenv and hands
// back a weak pointer to it. The caller discards the strong string return, so
// only the taint registry's own pinned owner can keep the array reachable.
func case104TaintedBox() (string, weak.Pointer[byte]) {
	_ = os.Setenv("CASE_104_INPUT", case104Secret)
	dirty := os.Getenv("CASE_104_INPUT")
	return dirty, weak.Make(unsafe.StringData(dirty))
}

func init() {
	register(Case{
		ID:   104,
		Name: "gc sweep clears heap shadow",
		Run: func() {
			_, pointer := case104TaintedBox()
			for cycle := 0; cycle < 10 && pointer.Value() != nil; cycle++ {
				runtime.GC()
			}
			surviving := pointer.Value()
			if surviving == nil {
				// Swept: the backing array was reclaimed, so there are no
				// surviving tainted bytes to drive into the sink.
				_, _ = os.Open("case104-swept")
				return
			}
			// Not swept: the registry's pinned owner kept the array reachable
			// through every forced collection. Recover its bytes and drive
			// them into the sink to prove tracking is still intact.
			recovered := unsafe.String(surviving, len(case104Secret))
			_, _ = os.Open("case104-" + recovered)
		},
		// Empirically confirmed: across 10 forced runtime.GC() cycles the
		// registry's pinned owner keeps the backing array reachable, so it is
		// never swept and the sink observes the recovered secret intact.
		Want: []Expect{{Value: "case104-" + case104Secret, Ranges: []taint.Range{{Start: 8, End: 43}}}},
	})
}
