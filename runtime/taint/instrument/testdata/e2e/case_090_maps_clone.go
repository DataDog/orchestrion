// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package main

import (
	"maps"
	"os"

	"github.com/DataDog/orchestrion/runtime/taint"
)

func init() {
	register(Case{
		ID:   90,
		Name: "maps clone",
		Run: func() {
			_ = os.Setenv("CASE_090_DIRTY", "secret")
			source := os.Getenv("CASE_090_DIRTY")
			case090Store := map[string]string{
				"dirty": source,
				"clean": "clean-value",
			}
			case090Clone := maps.Clone(case090Store)
			_, _ = os.Open(case090Clone["dirty"])
		},
		Want: []Expect{{Value: "secret", Ranges: []taint.Range{{Start: 0, End: 6}}}},
	})
}
