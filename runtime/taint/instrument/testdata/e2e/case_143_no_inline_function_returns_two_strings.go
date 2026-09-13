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
func case143Pair(v string) (string, string) { return v, v }

func init() {
	register(Case{
		ID:   143,
		Name: "no-inline function returns two strings",
		Run: func() {
			_ = os.Setenv("CASE_143_INPUT", "secret")
			first, second := case143Pair(os.Getenv("CASE_143_INPUT"))
			_, _ = os.Open(first)
			_, _ = os.Open(second)
		},
		// Empirically confirmed via captured=[{os.Open secret [{0 6}]}
		// {os.Open secret [{0 6}]}]: both results alias the same backing
		// array as the sourced env var (the function only copies string
		// headers, never fresh storage), so the registry resolves full
		// taint for each independently. The second result is identical to
		// the first: same value, same exact range, own report.
		Want: []Expect{
			{Value: "secret", Ranges: []taint.Range{{Start: 0, End: 6}}},
			{Value: "secret", Ranges: []taint.Range{{Start: 0, End: 6}}},
		},
	})
}
