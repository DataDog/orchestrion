// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package main

import (
	"os"

	"github.com/DataDog/orchestrion/runtime/taint"
)

type case148Runes []rune

func init() {
	register(Case{
		ID:   148,
		Name: "named rune slice conversion",
		Run: func() {
			_ = os.Setenv("CASE_148_INPUT", "secret")
			source := os.Getenv("CASE_148_INPUT")
			value := case148Runes(source)
			_, _ = os.Open("named-rune-" + string(value))
		},
		Want: []Expect{{Value: "named-rune-secret", Ranges: []taint.Range{{Start: 11, End: 17}}}},
	})
}
