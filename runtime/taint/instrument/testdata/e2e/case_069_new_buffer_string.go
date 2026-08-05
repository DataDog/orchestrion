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
		ID:   69,
		Name: "new buffer string",
		Run: func() {
			_ = os.Setenv("CASE_069_INPUT", "secret")
			source := os.Getenv("CASE_069_INPUT")
			buffer := bytes.NewBufferString(source)
			_, _ = os.Open("constructed-" + buffer.String())
		},
		Want: []Expect{{Value: "constructed-secret", Ranges: []taint.Range{{Start: 12, End: 18}}}},
	})
}
