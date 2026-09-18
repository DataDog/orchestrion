// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package main

import (
	"os"

	"github.com/DataDog/orchestrion/runtime/taint"
)

func init() {
	register(Case{
		ID:   88,
		Name: "map comma-ok lookup",
		Run: func() {
			_ = os.Setenv("CASE_088_DIRTY", "secret")
			source := os.Getenv("CASE_088_DIRTY")
			case088Store := map[string]string{"dirty": source}

			path, ok := case088Store["dirty"]
			if ok {
				_, _ = os.Open(path)
			}

			missing, found := case088Store["absent"]
			if !found {
				_, _ = os.Open(missing)
			}
		},
		Want: []Expect{{Value: "secret", Ranges: []taint.Range{{Start: 0, End: 6}}}},
	})
}
