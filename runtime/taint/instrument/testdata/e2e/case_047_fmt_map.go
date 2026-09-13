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
		ID:   47,
		Name: "fmt map",
		Run: func() {
			_ = os.Setenv("CASE_047_INPUT", "secret")
			source := os.Getenv("CASE_047_INPUT")
			_, _ = os.Open(fmt.Sprintf("map-%v", map[string]string{"value": source}))
		},
		Want: []Expect{{Value: "map-map[value:secret]", Ranges: []taint.Range{{Start: 0, End: 21}}}},
	})
}
