// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package main

import "os"

func main() {
	values := make(chan string, 1)
	blocked := make(chan string)
	values <- os.Getenv("TAINT_PATH")

	var path string
	select {
	case path = <-values:
	case path = <-blocked:
	}
	_, _ = os.Open(path)
}
