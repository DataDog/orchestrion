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
		ID:   62,
		Name: "buffer bytes view",
		Run: func() {
			_ = os.Setenv("CASE_062_INPUT", "secret")
			source := os.Getenv("CASE_062_INPUT")
			var buffer bytes.Buffer
			_, _ = buffer.Write([]byte(source))
			_, _ = os.Open("view-" + string(buffer.Bytes()))
		},
		Want: []Expect{{Value: "view-secret", Ranges: []taint.Range{{Start: 5, End: 11}}}},
	})
}
