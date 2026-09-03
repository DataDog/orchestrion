// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package main

import "os"

func main() {
	value := []byte(os.Getenv("TAINT_PATH"))
	if len(value) == 0 {
		value = []byte{0}
	}
	value[0] = 'X'
	_, _ = os.Open(string(value[:1]))
}
