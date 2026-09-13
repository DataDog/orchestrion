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
		ID:   73,
		Name: "buffer read from",
		Run: func() {
			_ = os.Setenv("CASE_073_INPUT", "secret")

			source := bytes.NewBufferString(os.Getenv("CASE_073_INPUT"))
			destination := bytes.NewBufferString("prefix-")
			_, _ = destination.ReadFrom(source)
			_, _ = os.Open(destination.String())
		},
		Want: []Expect{{Value: "prefix-secret", Ranges: []taint.Range{{Start: 7, End: 13}}}},
	})
}
