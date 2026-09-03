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
func case012RecoverPath(value string) (result string) {
	defer func() {
		if recover() != nil {
			result = os.Getenv("CASE_012_INPUT")
		}
	}()
	panic("case012 stop")
}

func init() {
	register(Case{
		ID:   12,
		Name: "recovered named return",
		Run: func() {
			_ = os.Setenv("CASE_012_INPUT", "secret")
			fn := case012RecoverPath
			_, _ = os.Open(fn("clean"))
		},
		Want: []Expect{{Value: "secret", Ranges: []taint.Range{{Start: 0, End: 6}}}},
	})
}
