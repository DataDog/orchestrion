// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package main

import "os"

func main() {
	const value = "secret"
	getenv := os.Getenv
	open := os.Open
	_, _ = open(value)
	_, _ = open(getenv("TAINT_PATH"))
}
