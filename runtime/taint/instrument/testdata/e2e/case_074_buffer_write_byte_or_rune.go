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
		ID:   74,
		Name: "buffer write byte or rune",
		Run: func() {
			_ = os.Setenv("CASE_074_INPUT", "secret")
			source := os.Getenv("CASE_074_INPUT")
			var first byte
			if len(source) > 0 {
				first = source[0]
			}

			var byteBuffer bytes.Buffer
			byteBuffer.WriteByte(first)
			_, _ = os.Open(byteBuffer.String())

			var runeBuffer bytes.Buffer
			runeBuffer.WriteRune(rune(first))
			_, _ = os.Open(runeBuffer.String())
		},
		Want: []Expect{
			{Value: "s", Ranges: []taint.Range{{Start: 0, End: 1}}},
			{Value: "s", Ranges: []taint.Range{{Start: 0, End: 1}}},
		},
	})
}
