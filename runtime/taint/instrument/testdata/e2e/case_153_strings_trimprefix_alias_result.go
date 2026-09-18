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
		ID:   153,
		Name: "strings.trimprefix alias result",
		Run: func() {
			_ = os.Setenv("CASE_153_INPUT", "prefix-secret")
			value := strings.TrimPrefix(os.Getenv("CASE_153_INPUT"), "prefix-")
			_, _ = os.Open(value)
		},
		Want: []Expect{{Value: "secret", Ranges: []taint.Range{{Start: 0, End: 6}}}},
	})
}
