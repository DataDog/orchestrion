// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

// Package hygiene holds caller bindings that occupy the generated names of the
// rewrite, plus a sibling file with package-scope declarations that do the
// same.
package hygiene

import "example.com/appendargs/lib"

// GeneratedNames occupies the generated base names of the slices and lib
// imports with locals in scope at the join point.
func GeneratedNames(args []lib.Option) (int, int) {
	__orchestrion_slices := 1
	__orchestrion_lib := 2
	_, _ = __orchestrion_slices, __orchestrion_lib
	lib.Sink(args...)
	return __orchestrion_slices, __orchestrion_lib
}

// SiblingValues exposes the package-scope declarations from sibling.go, so the
// tests can assert the caller values stay intact.
func SiblingValues() (int, int, int, int, int) {
	return slicesVar, optsVar, genSlices, genLib, appendVar
}
