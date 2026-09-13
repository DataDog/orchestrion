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
	b := bytes.NewBufferString("x")
	if len(source) > 0 {
		b.Bytes()[0] = source[0]
	}
	_, _ = os.Open(b.String())
}
