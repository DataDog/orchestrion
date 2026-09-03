// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package main

import "os"

const constantPath = "constant-" + "path"

func init() {
	register(Case{
		ID:   0,
		Name: "constant concat",
		Run: func() {
			_ = os.Setenv("CASE_000_CONSTANT_INPUT", "secret")
			_ = os.Getenv("CASE_000_CONSTANT_INPUT")
			_, _ = os.Open(constantPath)
		},
	})
}
