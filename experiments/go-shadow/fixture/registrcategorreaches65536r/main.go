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
	const liveCount = 65536

	values := make([]string, liveCount)
	for i := 0; i < liveCount; i++ {
		source := os.Getenv("TAINT_PATH")
		copy := source
		values[i] = copy
	}

	source := os.Getenv("TAINT_PATH")
	copy := source
	_, _ = os.Open(copy)

	runtime.KeepAlive(values)
}
