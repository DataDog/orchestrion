// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package main

import "os"

func init() {
	register(Case{
		ID:   31,
		Name: "clean byte overwrite",
		Run: func() {
			_ = os.Setenv("CASE_031_INPUT", "secret")
			source := os.Getenv("CASE_031_INPUT")
			value := []byte(source[0:1])
			value[0] = 'X'
			_, _ = os.Open(string(value))
		},
	})
}
