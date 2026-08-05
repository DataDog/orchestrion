// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package main

import (
	"encoding/xml"
	"os"

	"github.com/DataDog/orchestrion/runtime/taint"
)

type case135Payload struct {
	XMLName xml.Name `xml:"payload"`
	Path    string
}

func init() {
	register(Case{
		ID:   135,
		Name: "xml marshal bytes",
		Run: func() {
			clean, _ := xml.Marshal(case135Payload{Path: "clean"})
			_, _ = os.Open(string(clean))
			_ = os.Setenv("CASE_135_INPUT", "secret")
			data, _ := xml.Marshal(case135Payload{Path: os.Getenv("CASE_135_INPUT")})
			_, _ = os.Open(string(data))
		},
		Want: []Expect{{Value: "<payload><Path>secret</Path></payload>", Ranges: []taint.Range{{Start: 0, End: 38}}}},
	})
}
