// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package main

import "os"

func init() {
	register(Case{
		ID:   3,
		Name: "equal clean literal",
		Run: func() {
			_ = os.Setenv("CASE_003_INPUT", "secret")
			_ = os.Getenv("CASE_003_INPUT")
			_, _ = os.Open("secret")
		},
	})
}
