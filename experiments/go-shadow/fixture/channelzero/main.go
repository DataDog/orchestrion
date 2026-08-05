// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package main

import "runtime"

func main() {
	runtime.GC()
	var before runtime.MemStats
	runtime.ReadMemStats(&before)

	channel := make(chan struct{}, 1<<28)

	var after runtime.MemStats
	runtime.ReadMemStats(&after)
	if increase := after.HeapAlloc - before.HeapAlloc; increase > 8<<20 {
		panic("zero-sized channel allocated a proportional taint buffer")
	}
	runtime.KeepAlive(channel)
}
