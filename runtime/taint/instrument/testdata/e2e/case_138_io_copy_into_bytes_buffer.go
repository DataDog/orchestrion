// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package main

import (
	"bytes"
	"io"
	"os"
	"strings"

	"github.com/DataDog/orchestrion/runtime/taint"
)

func init() {
	register(Case{
		ID:   138,
		Name: "io copy into bytes buffer",
		Run: func() {
			var clean bytes.Buffer
			_, _ = io.Copy(&clean, strings.NewReader("clean"))
			_, _ = os.Open(clean.String())
			_ = os.Setenv("CASE_138_INPUT", "secret")
			var destination bytes.Buffer
			_, _ = io.Copy(&destination, strings.NewReader(os.Getenv("CASE_138_INPUT")))
			_, _ = os.Open(destination.String())
		},
		Want: []Expect{{Value: "secret", Ranges: []taint.Range{{Start: 0, End: 6}}}},
	})
}
