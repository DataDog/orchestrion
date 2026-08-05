// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package main

import "os"

type namedCopyData []byte
type namedCopyPath string

//go:noinline
func copyNamed(destination namedCopyData, source namedCopyPath) {
	copy(destination, source)
}

func main() {
	source := os.Getenv("TAINT_PATH")
	if source == "" {
		source = "/tmp/iast-namedstringcopy"
	}
	destination := make(namedCopyData, len(source))
	copyNamed(destination, namedCopyPath(source))
	_, _ = os.Open(string(destination))
}
