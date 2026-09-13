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
		ID:   79,
		Name: "select buffered send",
		Run: func() {
			_ = os.Setenv("CASE_079_INPUT", "secret")
			ch := make(chan string, 1)
			blocked := make(chan string)
			select {
			case ch <- os.Getenv("CASE_079_INPUT"):
			case blocked <- "clean":
			}
			received := <-ch
			_, _ = os.Open(received)
		},
		Want: []Expect{{Value: "secret", Ranges: []taint.Range{{Start: 0, End: 6}}}},
	})
}
