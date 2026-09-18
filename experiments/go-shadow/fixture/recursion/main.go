// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package main

import "os"

//go:noinline
func recurse(value string, depth int) string {
	if depth == 0 {
		return value
	}
	return recurse(value, depth-1)
}

func main() {
	_, _ = os.Open(recurse(os.Getenv("TAINT_PATH"), 200))
}
