// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package main

import (
	"os"
	"strconv"

	"github.com/DataDog/orchestrion/runtime/taint"
)

func init() {
	register(Case{
		ID:   87,
		Name: "map growth and rehash",
		Run: func() {
			_ = os.Setenv("CASE_087_INPUT", "secret")
			m := make(map[string]string)
			m["dirty"] = os.Getenv("CASE_087_INPUT")
			for i := 0; i < 1024; i++ {
				m[strconv.Itoa(i)] = "clean"
			}
			_, _ = os.Open(m["dirty"])
		},
		Want: []Expect{{Value: "secret", Ranges: []taint.Range{{Start: 0, End: 6}}}},
	})
}
