// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package main

import "os"

type octet byte
type octets []octet
type octetPath string

func main() {
	source := os.Getenv("TAINT_PATH")
	if source == "" {
		source = "/tmp/iast-namedoctets"
	}
	value := octets(source)
	_, _ = os.Open(string(octetPath(value)))
}
