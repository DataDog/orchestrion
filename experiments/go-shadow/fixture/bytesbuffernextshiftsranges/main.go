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
	var buf bytes.Buffer
	buf.WriteString("x")
	buf.WriteString(os.Getenv("TAINT_PATH"))
	buf.Next(1) // consume the clean leading byte, shifting the tainted offset
	_, _ = os.Open("next-" + buf.String())
}
