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
		ID:   77,
		Name: "unbuffered channel across goroutines",
		Run: func() {
			_ = os.Setenv("CASE_077_INPUT", "secret")
			channel := make(chan string)
			go func() {
				channel <- os.Getenv("CASE_077_INPUT")
			}()
			_, _ = os.Open(<-channel)
		},
		Want: []Expect{{Value: "secret", Ranges: []taint.Range{{Start: 0, End: 6}}}},
	})
}
