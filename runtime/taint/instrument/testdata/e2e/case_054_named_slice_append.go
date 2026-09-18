// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package main

import (
	"os"

	"github.com/DataDog/orchestrion/runtime/taint"
)

type appendDestination []byte
type appendSource []byte

func mixedAppend(destination appendDestination, source appendSource) appendDestination {
	return append(destination, source...)
}

func init() {
	register(Case{
		ID:   54,
		Name: "named slice append",
		Run: func() {
			_ = os.Setenv("CASE_054_INPUT", "secret")
			source := os.Getenv("CASE_054_INPUT")
			_, _ = os.Open(string(mixedAppend(appendDestination("mixed-"), appendSource(source))))
		},
		Want: []Expect{{Value: "mixed-secret", Ranges: []taint.Range{{Start: 6, End: 12}}}},
	})
}
