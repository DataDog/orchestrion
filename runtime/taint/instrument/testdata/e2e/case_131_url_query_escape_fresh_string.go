// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package main

import (
	"net/url"
	"os"

	"github.com/DataDog/orchestrion/runtime/taint"
)

func init() {
	register(Case{
		ID:   131,
		Name: "url query escape fresh string",
		Run: func() {
			_ = os.Setenv("CASE_131_INPUT", "/secret")
			_, _ = os.Open(url.QueryEscape("clean/path"))
			_, _ = os.Open(url.QueryEscape(os.Getenv("CASE_131_INPUT")))
		},
		Want: []Expect{{Value: "%2Fsecret", Ranges: []taint.Range{{Start: 0, End: 9}}}},
	})
}
