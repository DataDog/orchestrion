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
		Name: "buffer read string",
		Run: func() {
			_ = os.Setenv("CASE_073_INPUT", "secret")

			var buffer bytes.Buffer
			buffer.WriteString(os.Getenv("CASE_073_INPUT"))
			buffer.WriteString("\n")
			line, _ := buffer.ReadString('\n')
			_, _ = os.Open(line)
		},
		Want: []Expect{{Value: "secret\n", Ranges: []taint.Range{{Start: 0, End: 6}}}},
	})
}
