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
	const value = "alpha"
	_, _ = os.Open(strings.ToUpper(value + value))
	left := os.Getenv("TAINT_PATH")
	right := os.Getenv("TAINT_PATH")
	_, _ = os.Open(strings.ToUpper(left + right))
}
