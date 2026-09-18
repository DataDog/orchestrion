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
)

func main() {
	src := strings.NewReader(os.Getenv("TAINT_PATH"))
	var dst bytes.Buffer
	_, _ = io.Copy(&dst, src)
	_, _ = os.Open(dst.String())
}
