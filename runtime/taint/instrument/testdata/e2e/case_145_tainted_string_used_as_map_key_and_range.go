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
		ID:   145,
		Name: "tainted string used as map key and ranged back",
		Run: func() {
			_ = os.Setenv("CASE_145_INPUT", "secret")
			case145Store := map[string]struct{}{os.Getenv("CASE_145_INPUT"): {}}
			for key := range case145Store {
				_, _ = os.Open(key)
			}
		},
		Want: []Expect{{Value: "secret", Ranges: []taint.Range{{Start: 0, End: 6}}}},
	})
}
