// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package main

import (
	"os"

	"github.com/DataDog/orchestrion/runtime/taint"
)

type namedConversionPath string
type namedConversionData []byte

func init() {
	register(Case{
		ID:   52,
		Name: "named conversions",
		Run: func() {
			_ = os.Setenv("CASE_052_INPUT", "secret")
			source := os.Getenv("CASE_052_INPUT")
			named := namedConversionPath("named-") + namedConversionPath(source)
			value := namedConversionData(named)
			_, _ = os.Open(string(namedConversionPath(value)))
		},
		Want: []Expect{{Value: "named-secret", Ranges: []taint.Range{{Start: 6, End: 12}}}},
	})
}
