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
		ID:   24,
		Name: "string byte conversion",
		Run: func() {
			_ = os.Setenv("CASE_024_INPUT", "secret")
			source := os.Getenv("CASE_024_INPUT")
			joined := "safe-" + source
			data := []byte(joined)
			window := data[5:len(data)]
			copied := make([]byte, len(window))
			copy(copied, window)
			copied = append([]byte{}, copied...)
			path := string(copied)
			_, _ = os.Open(path[0:len(path)])
		},
		Want: []Expect{{Value: "secret", Ranges: []taint.Range{{Start: 0, End: 6}}}},
	})
}
