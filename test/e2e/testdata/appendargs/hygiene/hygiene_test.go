// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package hygiene

import (
	"testing"

	"example.com/appendargs/check"
	"example.com/appendargs/lib"
)

// TestHygieneGeneratedNames is H11: the locals that occupy the generated base
// names keep their values, and the woven call appends its value.
func TestHygieneGeneratedNames(t *testing.T) {
	lib.Record.Reset()
	a, b := GeneratedNames([]lib.Option{lib.Named("x")})
	check.LastNames(t, "x", "traced")
	if a != 1 || b != 2 {
		t.Fatalf("caller values changed: %d, %d", a, b)
	}
}

// TestHygienePackageScopeSibling is H24: the package-scope declarations of the
// sibling file stay intact through the weaving.
func TestHygienePackageScopeSibling(t *testing.T) {
	lib.Record.Reset()
	a, b, c, d, e := SiblingValues()
	want := []int{1, 2, 3, 4, 5}
	for i, g := range []int{a, b, c, d, e} {
		if g != want[i] {
			t.Fatalf("package-scope value %d = %d, want %d", i, g, want[i])
		}
	}
}
