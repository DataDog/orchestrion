// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package main

import (
	"os"
	"runtime"
)

func main() {
	values := make(chan string, 1)
	values <- os.Getenv("TAINT_PATH")

	pressure := make([][]byte, 64)
	for index := range pressure {
		pressure[index] = make([]byte, 1<<20)
	}
	runtime.GC()

	_, _ = os.Open(<-values)
	runtime.KeepAlive(pressure)
}
