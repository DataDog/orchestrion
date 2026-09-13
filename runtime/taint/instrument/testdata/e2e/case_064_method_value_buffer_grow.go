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
		ID:   64,
		Name: "method-value buffer grow",
		Run: func() {
			_ = os.Setenv("CASE_064_INPUT", "secret")

			var buffer bytes.Buffer
			buffer.WriteString(os.Getenv("CASE_064_INPUT"))
			grow := buffer.Grow
			grow(1 << 20)
			_, _ = os.Open(buffer.String())
		},
		Want: []Expect{{Value: "secret", Ranges: []taint.Range{{Start: 0, End: 6}}}},
	})
}
