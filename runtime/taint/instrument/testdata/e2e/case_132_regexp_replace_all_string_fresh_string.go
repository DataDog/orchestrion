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
		ID:   132,
		Name: "regexp replaceallstring fresh string",
		Run: func() {
			expression := regexp.MustCompile("value")
			_, _ = os.Open(expression.ReplaceAllString("clean-value", "public"))
			_ = os.Setenv("CASE_132_INPUT", "secret-value")
			_, _ = os.Open(expression.ReplaceAllString(os.Getenv("CASE_132_INPUT"), "public"))
		},
		Want: []Expect{{Value: "secret-public", Ranges: []taint.Range{{Start: 0, End: 13}}}},
	})
}
