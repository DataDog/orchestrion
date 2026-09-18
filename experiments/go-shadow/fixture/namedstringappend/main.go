// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package main

import "os"

type namedAppendData []byte
type namedAppendPath string

//go:noinline
func appendNamed(destination namedAppendData, source namedAppendPath) namedAppendData {
	return append(destination, source...)
}

func main() {
	source := os.Getenv("TAINT_PATH")
	if source == "" {
		source = "/tmp/iast-namedstringappend"
	}
	result := appendNamed(namedAppendData("prefix-"), namedAppendPath(source))
	_, _ = os.Open(string(result))
}
