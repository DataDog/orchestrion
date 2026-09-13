// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package main

import "os"

//go:noinline
func duplicate(path string) (string, string) {
	return path, path
}

func main() {
	const cleanPath = "/tmp/iast-noinline-function-returns-two-strings"
	cleanFirst, cleanSecond := duplicate(cleanPath)
	_, _ = os.Open(cleanFirst)
	_, _ = os.Open(cleanSecond)
	dirtyFirst, dirtySecond := duplicate(os.Getenv("TAINT_PATH"))
	_, _ = os.Open(dirtyFirst)
	_, _ = os.Open(dirtySecond)
}
