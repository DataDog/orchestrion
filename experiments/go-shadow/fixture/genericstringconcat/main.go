// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package main

import "os"

type genericString string

//go:noinline
func join[T ~string](left, right T) T {
	return left + right
}

func main() {
	source := os.Getenv("TAINT_PATH")
	if source == "" {
		source = "/tmp/iast-genericstringconcat"
	}
	value := join(genericString("generic-prefix-"), genericString(source))
	_, _ = os.Open(string(value))
}
