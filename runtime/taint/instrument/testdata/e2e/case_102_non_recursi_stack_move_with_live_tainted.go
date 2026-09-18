// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package main

import (
	"os"
	"runtime"

	"github.com/DataDog/orchestrion/runtime/taint"
)

//go:noinline
func case102ForceStackGrowth() {
	var frame [64 << 10]byte
	runtime.KeepAlive(&frame)
}

func init() {
	register(Case{
		ID:   102,
		Name: "non-recursive stack move with live tainted frame",
		Run: func() {
			_ = os.Setenv("CASE_102_INPUT", "secret")
			path := os.Getenv("CASE_102_INPUT")
			case102ForceStackGrowth()
			_, _ = os.Open(path)
		},
		Want: []Expect{{Value: "secret", Ranges: []taint.Range{{Start: 0, End: 6}}}},
	})
}
