// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package main

import (
	"os"

	"github.com/DataDog/orchestrion/runtime/taint"
)

type case094Key struct{ n int }

func init() {
	register(Case{
		ID:   94,
		Name: "generic small-key map",
		Run: func() {
			_ = os.Setenv("CASE_094_INPUT", "secret")
			case094Map := map[case094Key]string{{1}: os.Getenv("CASE_094_INPUT")}
			_, _ = os.Open(case094Map[case094Key{1}])
		},
		Want: []Expect{{Value: "secret", Ranges: []taint.Range{{Start: 0, End: 6}}}},
	})
}
