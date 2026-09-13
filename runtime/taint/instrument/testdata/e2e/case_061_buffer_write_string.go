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
		ID:   61,
		Name: "buffer write string",
		Run: func() {
			_ = os.Setenv("CASE_061_INPUT", "secret")
			source := os.Getenv("CASE_061_INPUT")
			var buffer bytes.Buffer
			_, _ = buffer.WriteString("buffer-")
			_, _ = buffer.WriteString(source)
			_, _ = os.Open(buffer.String())
		},
		Want: []Expect{{Value: "buffer-secret", Ranges: []taint.Range{{Start: 7, End: 13}}}},
	})
}
