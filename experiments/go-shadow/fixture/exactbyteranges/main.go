// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package main

import "os"

func main() {
	const clean = "secret"
	_, _ = os.Open("pre:" + clean + ":post")
	_, _ = os.Open("pre:" + os.Getenv("TAINT_PATH") + ":post")
}
