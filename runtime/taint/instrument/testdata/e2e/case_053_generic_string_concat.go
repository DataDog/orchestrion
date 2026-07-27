// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package main

import (
	"os"

	"github.com/DataDog/orchestrion/runtime/taint"
)

type genericPath string

func genericJoin[T ~string](left, right T) T {
	return left + right
}

func init() {
	register(Case{
		ID:   53,
		Name: "generic string concat",
		Run: func() {
			_ = os.Setenv("CASE_053_INPUT", "secret")
			source := os.Getenv("CASE_053_INPUT")
			_, _ = os.Open(string(genericJoin(genericPath("generic-"), genericPath(source))))
		},
		Want: []Expect{{Value: "generic-secret", Ranges: []taint.Range{{Start: 8, End: 14}}}},
	})
}
