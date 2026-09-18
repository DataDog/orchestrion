// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package main

import (
	"os"

	"github.com/DataDog/orchestrion/runtime/taint"
)

type namedCopyData []byte
type namedCopyPath string

func init() {
	register(Case{
		ID:   56,
		Name: "named copy",
		Run: func() {
			_ = os.Setenv("CASE_056_INPUT", "secret")
			source := os.Getenv("CASE_056_INPUT")
			value := make(namedCopyData, len(source))
			copy(value, namedCopyPath(source))
			_, _ = os.Open("named-copy-" + string(value))
		},
		Want: []Expect{{Value: "named-copy-secret", Ranges: []taint.Range{{Start: 11, End: 17}}}},
	})
}
