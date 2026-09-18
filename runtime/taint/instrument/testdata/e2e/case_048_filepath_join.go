// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package main

import (
	"os"
	"path/filepath"

	"github.com/DataDog/orchestrion/runtime/taint"
)

func init() {
	register(Case{
		ID:   48,
		Name: "filepath join",
		Run: func() {
			_ = os.Setenv("CASE_048_INPUT", "secret")
			source := os.Getenv("CASE_048_INPUT")
			_, _ = os.Open(filepath.Join("/tmp", source))
		},
		Want: []Expect{{Value: "/tmp/secret", Ranges: []taint.Range{{Start: 0, End: 11}}}},
	})
}
