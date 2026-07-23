// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package main

import "os"

//go:noinline
func hop(value string) (result string) {
	result = value
	return
}

func main() {
	function := hop
	_, _ = os.Open(function(os.Getenv("TAINT_PATH")))
}
