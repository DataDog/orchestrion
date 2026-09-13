// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package main

import "os"

type case148Runes []rune

func main() {
	const path = "/tmp/iast-named-rune-slice-conversion"
	_, _ = os.Open(string(case148Runes(path)))
	_, _ = os.Open(string(case148Runes(os.Getenv("TAINT_PATH"))))
}
