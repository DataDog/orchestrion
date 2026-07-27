// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package main

import (
	"bytes"
	"os"

	"github.com/DataDog/orchestrion/runtime/taint"
)

type cloneData []byte

func init() {
	register(Case{
		ID:   38,
		Name: "bytes clone",
		Run: func() {
			_ = os.Setenv("CASE_038_INPUT", "secret")
			source := os.Getenv("CASE_038_INPUT")
			cloned := bytes.Clone([]byte(source))
			_, _ = os.Open(string(append([]byte("bytes-"), cloned...)))
			typedClone := bytes.Clone(cloneData("x"))
			var _ *[]byte = &typedClone
		},
		Want: []Expect{{Value: "bytes-secret", Ranges: []taint.Range{{Start: 6, End: 12}}}},
	})
}
