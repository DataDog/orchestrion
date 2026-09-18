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
	src := bytes.NewBufferString(os.Getenv("TAINT_PATH"))
	var dst bytes.Buffer
	_, _ = src.WriteTo(&dst)
	_, _ = os.Open(dst.String())
}
