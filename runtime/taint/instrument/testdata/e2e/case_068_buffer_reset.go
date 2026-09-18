// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package main

import (
	"bytes"
	"os"
)

func init() {
	register(Case{
		ID:   68,
		Name: "buffer reset",
		Run: func() {
			_ = os.Setenv("CASE_068_INPUT", "secret")
			source := os.Getenv("CASE_068_INPUT")
			var buffer bytes.Buffer
			_, _ = buffer.Write([]byte(source))
			buffer.Reset()
			_, _ = buffer.WriteString("clean")
			_, _ = os.Open(buffer.String())
		},
	})
}
