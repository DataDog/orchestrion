// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package main

import "os"

func init() {
	register(Case{
		ID:   92,
		Name: "map delete",
		Run: func() {
			_ = os.Setenv("CASE_092_INPUT", "secret")
			case092Store := make(map[string]string)
			case092Store["key"] = os.Getenv("CASE_092_INPUT")
			delete(case092Store, "key")
			_, _ = os.Open(case092Store["key"])
		},
	})
}
