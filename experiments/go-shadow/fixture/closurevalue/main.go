// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package main

import (
	"os"
	"runtime"
)

//go:noinline
func makeOpener() func() {
	path := os.Getenv("TAINT_PATH")
	return func() {
		_, _ = os.Open(path)
	}
}

func main() {
	opener := makeOpener()
	runtime.GC()
	opener()
}
