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
	replacement := os.Getenv("TAINT_PATH")
	value := strings.Replace("x-value", "value", replacement, 1)
	_, _ = os.Open(value)
}
