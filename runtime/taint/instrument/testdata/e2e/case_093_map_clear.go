// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package main

import "os"

func init() {
	register(Case{
		ID:   93,
		Name: "map clear",
		Run: func() {
			_ = os.Setenv("CASE_093_INPUT", "secret")
			case093Store := make(map[string]string)
			case093Store["key"] = os.Getenv("CASE_093_INPUT")
			clear(case093Store)
			_, _ = os.Open(case093Store["key"])
		},
		// Empirically confirmed via captured=[]: clear(m) resets every entry
		// to its zero value, so the "key" lookup yields "". The empty string
		// has no backing-array address to look up in the registry, so no
		// report fires. Want is omitted (no report expected at all).
	})
}
