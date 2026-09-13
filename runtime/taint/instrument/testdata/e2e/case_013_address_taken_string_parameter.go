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
func case013Hop(v string) string {
	p := &v
	return *p
}

func init() {
	register(Case{
		ID:   13,
		Name: "address-taken string parameter",
		Run: func() {
			_ = os.Setenv("CASE_013_INPUT", "secret")
			_, _ = os.Open(case013Hop(os.Getenv("CASE_013_INPUT")))
		},
		Want: []Expect{{Value: "secret", Ranges: []taint.Range{{Start: 0, End: 6}}}},
	})
}
