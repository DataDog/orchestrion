// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package main

import "os"

//go:noinline
func recovered(value string) (result string) {
	defer func() {
		if recover() != nil {
			result = os.Getenv("TAINT_PATH")
		}
	}()
	panic("recover")
}

func main() {
	function := recovered
	_, _ = os.Open(function("clean"))
}
