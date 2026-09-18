// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package main

import (
	"os"
	"strings"

	"github.com/DataDog/orchestrion/runtime/taint"
)

func init() {
	register(Case{
		ID:   152,
		Name: "strings.cut alias result",
		Run: func() {
			_ = os.Setenv("CASE_152_INPUT", "prefix/secret")
			_, after, found := strings.Cut(os.Getenv("CASE_152_INPUT"), "/")
			if found {
				_, _ = os.Open(after)
			}
		},
		Want: []Expect{{Value: "secret", Ranges: []taint.Range{{Start: 0, End: 6}}}},
	})
}
