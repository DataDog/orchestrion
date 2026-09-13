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

func init() {
	register(Case{
		ID:   139,
		Name: "buffer write to bytes buffer",
		Run: func() {
			cleanPointerDestination := bytes.NewBufferString("pointer-clean-")
			_, _ = bytes.NewBufferString("secret").WriteTo(cleanPointerDestination)
			_, _ = os.Open(cleanPointerDestination.String())
			_ = os.Setenv("CASE_139_INPUT", "secret")
			tainted := os.Getenv("CASE_139_INPUT")
			pointerDestination := bytes.NewBufferString("pointer-dirty-")
			_, _ = bytes.NewBufferString(tainted).WriteTo(pointerDestination)
			_, _ = os.Open(pointerDestination.String())

			cleanValueSource := *bytes.NewBufferString("secret")
			cleanValueDestination := *bytes.NewBufferString("value-clean-")
			_, _ = cleanValueSource.WriteTo(&cleanValueDestination)
			_, _ = os.Open(cleanValueDestination.String())

			valueSource := *bytes.NewBufferString(tainted)
			valueDestination := *bytes.NewBufferString("value-dirty-")
			_, _ = valueSource.WriteTo(&valueDestination)
			_, _ = os.Open(valueDestination.String())
		},
		Want: []Expect{
			{Value: "pointer-dirty-secret", Ranges: []taint.Range{{Start: 14, End: 20}}},
			{Value: "value-dirty-secret", Ranges: []taint.Range{{Start: 12, End: 18}}},
		},
	})
}
