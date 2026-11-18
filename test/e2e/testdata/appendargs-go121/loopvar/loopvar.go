// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

// Package loopvar guards the language version request of the spread rewrite
// (V06): under the go1.21 language, the addresses collected from a classic
// three-clause for loop are all equal. If the rewrite requested go1.22, the
// compiler would switch to per-iteration loop variables and they would differ.
package loopvar

import "example.com/appendargs-go121/local"

// WovenAndPtrs both spreads into the woven sink and collects the addresses of
// the loop variable, so the package's compile flags cover both.
func WovenAndPtrs(xs []local.Opt) []*int {
	local.Sink(xs...)
	var out []*int
	for i := 0; i < 3; i++ {
		out = append(out, &i)
	}
	return out
}
