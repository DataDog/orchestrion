// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package main

import "os"

func main() {
	source := os.Getenv("TAINT_PATH")

	data := []byte(source)
	_, _ = os.Open(string(data))

	end := len(data)
	if end > 3 {
		end = 3
	}
	window := data[:end:end]
	_, _ = os.Open(string(window))
}
