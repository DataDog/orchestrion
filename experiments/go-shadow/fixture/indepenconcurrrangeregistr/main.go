// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package main

import (
	"os"
	"sync"
)

const goroutineCount = 32

func main() {
	var wg sync.WaitGroup
	wg.Add(goroutineCount)
	for i := 0; i < goroutineCount; i++ {
		go func() {
			defer wg.Done()
			path := os.Getenv("TAINT_PATH")
			_, _ = os.Open(path)
		}()
	}
	wg.Wait()
}
