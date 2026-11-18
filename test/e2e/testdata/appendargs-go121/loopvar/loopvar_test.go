// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package loopvar

import (
	"testing"

	"example.com/appendargs-go121/local"
)

// TestLoopVarGo121 is V06: under the module's go1.21 language, all collected
// addresses are equal, and the woven call still appends its value.
func TestLoopVarGo121(t *testing.T) {
	local.Reset()
	ptrs := WovenAndPtrs([]local.Opt{local.Named("x")})
	if len(ptrs) != 3 {
		t.Fatalf("collected %d pointers, want 3", len(ptrs))
	}
	for i := 1; i < len(ptrs); i++ {
		if ptrs[i] != ptrs[0] {
			t.Fatalf("pointer %d differs: the module was compiled with per-iteration loop variables", i)
		}
	}
	calls := local.Record()
	if len(calls) != 1 {
		t.Fatalf("recorded %v", calls)
	}
	got := calls[0]
	if len(got) != 2 || got[0] != "x" || got[1] != "traced" {
		t.Fatalf("recorded %v, want [x traced]", got)
	}
}
