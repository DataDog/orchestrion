// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package main

import "os"

func init() {
	register(Case{
		ID:   91,
		Name: "map overwrite with clean value",
		Run: func() {
			_ = os.Setenv("CASE_091_INPUT", "secret")
			case091Store := map[string]string{}
			case091Store["key"] = os.Getenv("CASE_091_INPUT")
			case091Store["key"] = "clean"
			_, _ = os.Open(case091Store["key"])
		},
	})
}
