// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package main

import "os"

type genericByteSlice []byte

func appendGeneric[S ~[]E, E ~byte](value S, suffix E) S {
	return append(value, suffix)
}

func main() {
	source := os.Getenv("TAINT_PATH")
	if source == "" {
		source = "/tmp/iast-genericbytesliceappend"
	}
	result := appendGeneric(genericByteSlice(source), '!')
	_, _ = os.Open(string(result))
}
