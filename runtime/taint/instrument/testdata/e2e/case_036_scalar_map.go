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
		ID:   36,
		Name: "scalar map",
		Run: func() {
			_ = os.Setenv("CASE_036_INPUT", "secret")
			source := os.Getenv("CASE_036_INPUT")
			byteValue := source[0]
			runeValue := []rune(source)[0]
			byteValues := make(map[string]byte)
			runeValues := make(map[string]rune)
			byteValues["value"] = byteValue
			runeValues["value"] = runeValue
			var byteResult byte
			var runeResult rune
			byteResult = byteValues["value"]
			runeResult = runeValues["value"]
			_, _ = os.Open(string(byteResult))
			_, _ = os.Open(string(runeResult))
			cleanByteValue := byte('c')
			cleanRuneValue := rune('c')
			byteValues["value"] = cleanByteValue
			runeValues["value"] = cleanRuneValue
			var cleanByte byte
			var cleanRune rune
			cleanByte = byteValues["value"]
			cleanRune = runeValues["value"]
			_, _ = os.Open(string(cleanByte))
			_, _ = os.Open(string(cleanRune))
		},
		Want: []Expect{
			{Value: "s", Ranges: []taint.Range{{Start: 0, End: 1}}},
			{Value: "s", Ranges: []taint.Range{{Start: 0, End: 1}}},
		},
	})
}
