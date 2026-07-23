// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package main

import "os"

//go:noinline
func ignore(string) string { return "clean" }

//go:noinline
func ignoreBlank(_ string) string { return "clean" }

func main() {
	_, _ = os.Open(ignore(os.Getenv("TAINT_PATH")))
	_, _ = os.Open(ignoreBlank(os.Getenv("TAINT_PATH")))
}
