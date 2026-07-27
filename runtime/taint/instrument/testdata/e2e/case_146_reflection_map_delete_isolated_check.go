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
		ID:   146,
		Name: "reflection map delete isolated check",
		Run: func() {
			_ = os.Setenv("CASE_146_INPUT", "secret")
			case146Source := os.Getenv("CASE_146_INPUT")
			case146Key := "dirty"
			case146Map := map[string]string{case146Key: case146Source}
			case146Value := reflect.ValueOf(case146Map)
			case146Value.SetMapIndex(reflect.ValueOf(case146Key), reflect.Value{})
			_, _ = os.Open(case146Map[case146Key])
		},
		// Empirically confirmed via a deliberately wrong probe: captured=[]
		// (zero reports). This isolates the DELETE half of case 95: reflect.
		// Value.SetMapIndex(key, reflect.Value{}) is untouched by every
		// orchestrion.yml aspect (no join point matches reflect.Value methods
		// or map operations), so it runs as a plain runtime map delete. The
		// deleted key drops the "secret" backing-pointer association, so the
		// subsequent m[key] lookup yields the zero value "" for a string -
		// an empty string has no backing array to key the registry on, so
		// os.Open("") finds no taint. Correct clean isolation, no stale-taint
		// false positive. Want is omitted (no report expected at all).
	})
}
