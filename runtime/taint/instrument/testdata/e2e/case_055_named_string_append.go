// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package main

import (
	"os"

	"github.com/DataDog/orchestrion/runtime/taint"
)

type namedAppendData []byte
type namedAppendPath string

func namedStringAppend(destination namedAppendData, source namedAppendPath) namedAppendData {
	return append(destination, source...)
}

func init() {
	register(Case{
		ID:   55,
		Name: "named string append",
		Run: func() {
			_ = os.Setenv("CASE_055_INPUT", "secret")
			source := os.Getenv("CASE_055_INPUT")
			value := namedStringAppend(namedAppendData("named-append-"), namedAppendPath(source))
			_, _ = os.Open(string(value))
		},
		Want: []Expect{{Value: "named-append-secret", Ranges: []taint.Range{{Start: 13, End: 19}}}},
	})
}
