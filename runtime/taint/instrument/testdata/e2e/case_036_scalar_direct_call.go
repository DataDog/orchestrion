// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package main

import (
	"os"

	"github.com/DataDog/orchestrion/runtime/taint"
)

func case036ByteIdentity(value byte) byte { return value }

func case036RuneIdentity(value rune) rune { return value }

func init() {
	register(Case{
		ID:   36,
		Name: "scalar direct call",
		Run: func() {
			_ = os.Setenv("CASE_036_INPUT", "secret")
			source := os.Getenv("CASE_036_INPUT")
			byteValue := source[0]
			runeValue := []rune(source)[0]
			var byteResult byte
			var runeResult rune
			byteResult = case036ByteIdentity(byteValue)
			runeResult = case036RuneIdentity(runeValue)
			_, _ = os.Open(string(byteResult))
			_, _ = os.Open(string(runeResult))
		},
		Want: []Expect{
			{Value: "s", Ranges: []taint.Range{{Start: 0, End: 1}}},
			{Value: "s", Ranges: []taint.Range{{Start: 0, End: 1}}},
		},
	})
}
