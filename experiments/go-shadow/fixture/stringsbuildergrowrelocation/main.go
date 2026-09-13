// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package main

import (
	"os"
	"strings"
)

func main() {
	var b strings.Builder
	b.WriteString(os.Getenv("TAINT_PATH"))
	b.Grow(1 << 20)
	_, _ = os.Open(b.String())
}
