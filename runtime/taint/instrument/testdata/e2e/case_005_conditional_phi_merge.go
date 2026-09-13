// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package main

import (
	"os"

	"github.com/DataDog/orchestrion/runtime/taint"
)

func init() {
	register(Case{
		ID:   5,
		Name: "conditional phi merge",
		Run: func() {
			_ = os.Setenv("CASE_005_TAKE_DIRTY_BRANCH", "1")
			_ = os.Setenv("CASE_005_INPUT", "secret")
			useDirty := os.Getenv("CASE_005_TAKE_DIRTY_BRANCH") != ""
			path := "clean"
			if useDirty {
				path = os.Getenv("CASE_005_INPUT")
			}
			_, _ = os.Open(path)
		},
		Want: []Expect{{Value: "secret", Ranges: []taint.Range{{Start: 0, End: 6}}}},
	})
}
