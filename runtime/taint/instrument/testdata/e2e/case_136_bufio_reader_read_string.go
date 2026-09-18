// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package main

import (
	"bufio"
	"os"
	"strings"

	"github.com/DataDog/orchestrion/runtime/taint"
)

func init() {
	register(Case{
		ID:   136,
		Name: "bufio reader read string",
		Run: func() {
			clean, _ := bufio.NewReader(strings.NewReader("clean\n")).ReadString('\n')
			_, _ = os.Open(clean)
			_ = os.Setenv("CASE_136_INPUT", "secret")
			result, _ := bufio.NewReader(strings.NewReader(os.Getenv("CASE_136_INPUT") + "\n")).ReadString('\n')
			_, _ = os.Open(result)
		},
		Want: []Expect{{Value: "secret\n", Ranges: []taint.Range{{Start: 0, End: 6}}}},
	})
}
