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
func case011Hop(value string) string { return value }

func init() {
	register(Case{
		ID:   11,
		Name: "panic/recover transition cleanup",
		Run: func() {
			_ = os.Setenv("CASE_011_INPUT", "secret")
			panicking := func(value string) string { panic(value) }
			for range 100 {
				func() {
					defer func() { _ = recover() }()
					_ = panicking(os.Getenv("CASE_011_INPUT"))
				}()
			}
			fn := case011Hop
			_, _ = os.Open(fn(os.Getenv("CASE_011_INPUT")))
		},
		Want: []Expect{{Value: "secret", Ranges: []taint.Range{{Start: 0, End: 6}}}},
	})
}
