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
	if len(source) == 0 {
		source = "x"
	}
	b := bytes.NewBufferString(source[:1])
	b.Bytes()[0] = 'X'
	_, _ = os.Open(b.String())
}
