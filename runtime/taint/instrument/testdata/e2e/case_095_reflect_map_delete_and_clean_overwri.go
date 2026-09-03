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
		ID:   95,
		Name: "reflection map delete and clean overwrite",
		Run: func() {
			_ = os.Setenv("CASE_095_INPUT", "secret")
			case095Source := os.Getenv("CASE_095_INPUT")
			case095Key := "key"
			case095Map := map[string]string{case095Key: case095Source}
			case095Value := reflect.ValueOf(case095Map)
			case095Value.SetMapIndex(reflect.ValueOf(case095Key), reflect.Value{})
			case095Value.SetMapIndex(reflect.ValueOf(case095Key), reflect.ValueOf("clean"))
			_, _ = os.Open(case095Map[case095Key])
		},
		// Empirically confirmed via a deliberately wrong probe: captured=[]
		// (zero reports). reflect.Value.SetMapIndex is untouched by every
		// orchestrion.yml aspect (no join point matches reflect.Value methods
		// or map operations at all), so both the delete and the overwrite run
		// as plain runtime map mutations with no taint-aware rewriting. Taint
		// propagation through maps has never depended on map-specific
		// instrumentation to begin with - it depends solely on the
		// registry's backing-array-address keying (see case 90/92/93) - so
		// the reflective path behaves identically to the direct-call path.
		// The deleted key drops the "secret" backing pointer, the reflective
		// overwrite installs the literal "clean" (a never-registered backing
		// array), and the plain os.Open(m[key]) sink lookup finds no taint:
		// correct clean isolation, no stale-taint false positive. Want is
		// omitted (no report expected at all).
	})
}
