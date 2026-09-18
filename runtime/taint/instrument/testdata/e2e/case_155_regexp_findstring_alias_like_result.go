// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package main

import (
	"os"
	"regexp"

	"github.com/DataDog/orchestrion/runtime/taint"
)

func init() {
	register(Case{
		ID:   155,
		Name: "regexp.findstring alias-like result",
		Run: func() {
			_ = os.Setenv("CASE_155_INPUT", "secret")
			re := regexp.MustCompile(".+")
			value := re.FindString(os.Getenv("CASE_155_INPUT"))
			_, _ = os.Open(value)
		},
		// FindString returns s[a[0]:a[1]], a plain re-slice of the input backing
		// array with no fresh allocation, so the registry's address-based
		// tracking finds the full tainted span even though regexp itself is
		// never instrumented (empirically confirmed: captured value "secret"
		// with ranges [{0 6}]).
		Want: []Expect{{Value: "secret", Ranges: []taint.Range{{Start: 0, End: 6}}}},
	})
}
