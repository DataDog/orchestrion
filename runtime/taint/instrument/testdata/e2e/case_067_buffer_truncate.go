// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package main

import (
	"bytes"
	"os"

	"github.com/DataDog/orchestrion/runtime/taint"
)

func init() {
	register(Case{
		ID:   67,
		Name: "buffer truncate",
		Run: func() {
			_ = os.Setenv("CASE_067_INPUT", "secret")
			source := os.Getenv("CASE_067_INPUT")
			var buffer bytes.Buffer
			_, _ = buffer.Write([]byte(source))
			buffer.Truncate(3)
			_, _ = os.Open("truncate-" + buffer.String())
		},
		Want: []Expect{{Value: "truncate-sec", Ranges: []taint.Range{{Start: 9, End: 12}}}},
	})
}
