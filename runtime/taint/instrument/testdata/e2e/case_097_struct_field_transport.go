// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package main

import (
	"os"

	"github.com/DataDog/orchestrion/runtime/taint"
)

type case097Box struct {
	path  string
	label string
}

func init() {
	register(Case{
		ID:   97,
		Name: "struct field transport",
		Run: func() {
			_ = os.Setenv("CASE_097_INPUT", "secret")
			box := &case097Box{path: os.Getenv("CASE_097_INPUT"), label: "case097-clean"}
			_, _ = os.Open(box.path)
			_, _ = os.Open(box.label)
		},
		// Empirically confirmed via captured=[{os.Open secret [{0 6}]}]: the
		// tainted string header is copied into the field by the composite
		// literal and copied back out by the field read, with no allocation
		// in between, so the registry's backing-array-address lookup still
		// matches. The second os.Open call reads the clean label field and
		// produces no report, so Want has exactly one entry.
		Want: []Expect{{Value: "secret", Ranges: []taint.Range{{Start: 0, End: 6}}}},
	})
}
