// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package main

import (
	"os"

	"github.com/DataDog/orchestrion/runtime/taint"
)

type genericAppendData []byte

func genericGrow[S ~[]E, E ~byte](value S, element E) S {
	return append(value, element)
}

func init() {
	register(Case{
		ID:   28,
		Name: "generic byte append",
		Run: func() {
			_ = os.Setenv("CASE_028_INPUT", "secret")
			source := os.Getenv("CASE_028_INPUT")
			value := genericGrow(genericAppendData(source), byte('!'))
			_, _ = os.Open("generic-bytes-" + string(value))
		},
		Want: []Expect{{Value: "generic-bytes-secret!", Ranges: []taint.Range{{Start: 14, End: 20}}}},
	})
}
