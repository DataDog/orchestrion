// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package main

import (
	"os"
	"reflect"
)

func init() {
	register(Case{
		ID:   147,
		Name: "reflection map clean overwrite isolated check",
		Run: func() {
			_ = os.Setenv("CASE_147_INPUT", "secret")
			case147Key := "dirty"
			case147Source := os.Getenv("CASE_147_INPUT")
			case147Map := map[string]string{case147Key: case147Source}
			case147Value := reflect.ValueOf(case147Map)
			case147Value.SetMapIndex(reflect.ValueOf(case147Key), reflect.ValueOf("clean"))
			_, _ = os.Open(case147Map[case147Key])
		},
		// Empirically confirmed via a deliberately wrong probe: captured=[]
		// (zero reports). reflect.Value.SetMapIndex has no matching join
		// point in orchestrion.yml (no aspect touches reflect.Value methods
		// or map operations at all), so the overwrite runs as a plain
		// runtime map mutation. The map's backing header for key "dirty" is
		// replaced by a fresh header pointing at the "clean" string literal,
		// which was never registered in the taint registry (keyed by
		// backing-array address, see case 90/92/93). The subsequent
		// os.Open(m[key]) sink lookup therefore finds no taint: this is the
		// CORRECT outcome (the value genuinely is clean), not a tracking
		// gap. Want is omitted (no report expected); any report here would
		// be a stale-taint false positive.
	})
}
