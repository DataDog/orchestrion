// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package main

import "os"

func shadowedAppend(source []byte) []byte {
	append := func(_ []byte, _ ...byte) []byte { return []byte("clean") }
	return append(nil, source...)
}

func init() {
	register(Case{
		ID:   0,
		Name: "shadowed append",
		Run: func() {
			_ = os.Setenv("CASE_000_SHADOW_INPUT", "secret")
			source := os.Getenv("CASE_000_SHADOW_INPUT")
			_, _ = os.Open(string(shadowedAppend([]byte(source))))
		},
	})
}
