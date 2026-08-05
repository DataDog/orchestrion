// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package main

import (
	"fmt"
	"os"

	"github.com/DataDog/orchestrion/runtime/taint"
)

func init() {
	register(Case{
		ID:   46,
		Name: "fmt sprintf",
		Run: func() {
			_ = os.Setenv("CASE_046_INPUT", "secret")
			source := os.Getenv("CASE_046_INPUT")
			_, _ = os.Open(fmt.Sprintf("fmt-%s", source))
		},
		Want: []Expect{{Value: "fmt-secret", Ranges: []taint.Range{{Start: 0, End: 10}}}},
	})
}
