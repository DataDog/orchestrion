// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package main

import (
	"os"

	"github.com/DataDog/orchestrion/runtime/taint"
	"github.com/DataDog/orchestrion/runtime/taint/instrument/testdata/dependency"
)

func init() {
	register(Case{
		ID:   51,
		Name: "fresh allocation in uninstrumented dependency",
		Run: func() {
			_, _ = os.Open(dependency.Clone("clean"))
			_ = os.Setenv("CASE_051_INPUT", "secret")
			_, _ = os.Open(dependency.Clone(os.Getenv("CASE_051_INPUT")))
		},
		Want: []Expect{{Value: "secret", Ranges: []taint.Range{{Start: 0, End: 6}}}},
	})
}
