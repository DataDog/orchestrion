// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package main

import (
	"os"

	"github.com/DataDog/orchestrion/runtime/taint"
)

type octet byte
type octets []octet
type octetPath string

func init() {
	register(Case{
		ID:   57,
		Name: "named octets",
		Run: func() {
			_ = os.Setenv("CASE_057_INPUT", "secret")
			source := os.Getenv("CASE_057_INPUT")
			value := octets(source)
			_, _ = os.Open("octets-" + string(octetPath(value)))
		},
		Want: []Expect{{Value: "octets-secret", Ranges: []taint.Range{{Start: 7, End: 13}}}},
	})
}
