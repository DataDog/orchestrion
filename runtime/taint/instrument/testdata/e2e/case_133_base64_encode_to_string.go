// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package main

import (
	"encoding/base64"
	"os"

	"github.com/DataDog/orchestrion/runtime/taint"
)

func init() {
	register(Case{
		ID:   133,
		Name: "base64 encode to string",
		Run: func() {
			_, _ = os.Open(base64.StdEncoding.EncodeToString([]byte("clean")))
			_ = os.Setenv("CASE_133_INPUT", "secret")
			_, _ = os.Open(base64.StdEncoding.EncodeToString([]byte(os.Getenv("CASE_133_INPUT"))))
		},
		Want: []Expect{{Value: "c2VjcmV0", Ranges: []taint.Range{{Start: 0, End: 8}}}},
	})
}
