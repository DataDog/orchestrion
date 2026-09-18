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
		ID:   154,
		Name: "strings.split slice of string views",
		Run: func() {
			_ = os.Setenv("CASE_154_INPUT", "alpha-value/beta-value/gamma-value")
			case154Parts := strings.Split(os.Getenv("CASE_154_INPUT"), "/")
			_, _ = os.Open(case154Parts[0])
			_, _ = os.Open(case154Parts[1])
		},
		// strings.Split has no aspect (unlike strings.Cut in case 152); it runs
		// as plain stdlib code that reslices the sourced string header without
		// any fresh allocation. Each []string element is therefore a distinct
		// view at its own backing-array offset into the SAME tainted array:
		// case154Parts[0] starts at offset 0 (length 11), case154Parts[1] starts
		// at offset 12, right after "alpha-value/" (length 10). Empirically
		// confirmed via captured=[{os.Open alpha-value [{0 11}]} {os.Open
		// beta-value [{0 10}]}]: the registry resolves full taint independently
		// for each view. Both report {0, len(view)} relative to their own start
		// -- not because the offset is ignored, but because the entire sourced
		// string is tainted end-to-end, so every view lies fully inside the
		// tainted address window no matter where that window begins.
		Want: []Expect{
			{Value: "alpha-value", Ranges: []taint.Range{{Start: 0, End: 11}}},
			{Value: "beta-value", Ranges: []taint.Range{{Start: 0, End: 10}}},
		},
	})
}
