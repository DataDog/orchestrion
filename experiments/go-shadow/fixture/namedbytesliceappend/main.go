// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package main

import "os"

type appendDestination []byte
type appendSource []byte

//go:noinline
func mixedAppend(destination appendDestination, source appendSource) appendDestination {
	return append(destination, source...)
}

func main() {
	source := os.Getenv("TAINT_PATH")
	if source == "" {
		source = "/tmp/iast-namedbytesliceappend"
	}
	result := mixedAppend(appendDestination("prefix-"), appendSource(source))
	_, _ = os.Open(string(result))
}
