// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package main

import (
	"os"

	"github.com/DataDog/orchestrion/runtime/taint"
)

func init() {
	register(Case{
		ID:   122,
		Name: "os.OpenFile sink",
		Run: func() {
			_ = os.Setenv("CASE_122_INPUT", "secret")
			_, _ = os.OpenFile(os.Getenv("CASE_122_INPUT"), os.O_RDONLY, 0)
		},
		Want: []Expect{{Value: "secret", Ranges: []taint.Range{{Start: 0, End: 6}}}},
	})
}
