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
func case010Outer(value string) (result string) {
	result = value
	deferred := func(clean string) string { return clean }
	defer func() { _ = deferred("clean") }()
	return
}

func init() {
	register(Case{
		ID:   10,
		Name: "deferred dynamic call cleanup",
		Run: func() {
			_ = os.Setenv("CASE_010_INPUT", "secret")
			fn := case010Outer
			_, _ = os.Open(fn(os.Getenv("CASE_010_INPUT")))
		},
		Want: []Expect{{Value: "secret", Ranges: []taint.Range{{Start: 0, End: 6}}}},
	})
}
