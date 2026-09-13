// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package main

import (
	"bytes"
	"os"
)

func main() {
	source := os.Getenv("TAINT_PATH")

	first := byte('/')
	if len(source) > 0 {
		first = source[0]
	}

	var byteBuf bytes.Buffer
	byteBuf.WriteByte(first)
	_, _ = os.Open(byteBuf.String())

	var runeBuf bytes.Buffer
	runeBuf.WriteRune(rune(first))
	_, _ = os.Open(runeBuf.String())
}
