// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package main

import "os"

func init() {
	register(Case{
		ID:   30,
		Name: "clear bytes",
		Run: func() {
			_ = os.Setenv("CASE_030_INPUT", "secret")
			source := os.Getenv("CASE_030_INPUT")
			value := []byte(source)
			clear(value)
			_, _ = os.Open(string(value))
		},
	})
}
