// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package main

import "os"

func main() {
	const value = "secret"
	_, _ = os.Open(value + value)
	left := os.Getenv("TAINT_PATH")
	right := os.Getenv("TAINT_PATH")
	_, _ = os.Open(left + right)
}
