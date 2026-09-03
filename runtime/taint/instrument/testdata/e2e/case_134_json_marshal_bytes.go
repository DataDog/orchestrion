// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package main

import (
	"encoding/json"
	"os"

	"github.com/DataDog/orchestrion/runtime/taint"
)

func init() {
	register(Case{
		ID:   134,
		Name: "json marshal bytes",
		Run: func() {
			clean, _ := json.Marshal(map[string]string{"path": "clean"})
			_, _ = os.Open(string(clean))
			_ = os.Setenv("CASE_134_INPUT", "secret")
			data, _ := json.Marshal(map[string]string{"path": os.Getenv("CASE_134_INPUT")})
			_, _ = os.Open(string(data))
		},
		Want: []Expect{{Value: `{"path":"secret"}`, Ranges: []taint.Range{{Start: 0, End: 17}}}},
	})
}
