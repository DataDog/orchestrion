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
		ID:   71,
		Name: "buffer alias dirty write",
		Run: func() {
			_ = os.Setenv("CASE_071_INPUT", "secret")
			source := os.Getenv("CASE_071_INPUT")
			buffer := bytes.NewBufferString("x")
			view := buffer.Bytes()
			sourceBytes := []byte(source)
			view[0] = sourceBytes[0]
			_, _ = os.Open("alias-" + buffer.String())
		},
		Want: []Expect{{Value: "alias-s", Ranges: []taint.Range{{Start: 6, End: 7}}}},
	})
}
