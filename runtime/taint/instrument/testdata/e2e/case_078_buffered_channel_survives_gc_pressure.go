// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package main

import (
	"os"
	"runtime"

	"github.com/DataDog/orchestrion/runtime/taint"
)

func init() {
	register(Case{
		ID:   78,
		Name: "buffered channel survives gc pressure",
		Run: func() {
			_ = os.Setenv("CASE_078_INPUT", "secret")
			case078Channel := make(chan string, 1)
			case078Channel <- os.Getenv("CASE_078_INPUT")
			case078Pressure := make([][]byte, 64)
			for index := range case078Pressure {
				case078Pressure[index] = make([]byte, 1<<20)
			}
			runtime.GC()
			_, _ = os.Open(<-case078Channel)
			runtime.KeepAlive(case078Pressure)
		},
		Want: []Expect{{Value: "secret", Ranges: []taint.Range{{Start: 0, End: 6}}}},
	})
}
