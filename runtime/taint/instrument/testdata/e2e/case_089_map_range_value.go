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
		ID:   89,
		Name: "map range value",
		Run: func() {
			_ = os.Setenv("CASE_089_DIRTY", "secret")
			source := os.Getenv("CASE_089_DIRTY")
			case089Store := map[string]string{
				"dirty": source,
				"alpha": "clean-alpha",
				"beta":  "clean-beta",
			}

			for key, path := range case089Store {
				if key == "dirty" {
					_, _ = os.Open(path)
				}
			}
		},
		Want: []Expect{{Value: "secret", Ranges: []taint.Range{{Start: 0, End: 6}}}},
	})
}
