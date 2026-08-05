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
		ID:   72,
		Name: "buffer alias clean overwrite",
		Run: func() {
			_ = os.Setenv("CASE_072_INPUT", "secret")
			source := os.Getenv("CASE_072_INPUT")
			buffer := bytes.NewBufferString(source[0:1])
			buffer.Bytes()[0] = 'X'
			_, _ = os.Open(buffer.String())
		},
	})
}
