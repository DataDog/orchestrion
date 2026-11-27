// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package noop

import (
	"testing"

	"example.com/appendargs-go117/local"
)

func TestNoop(t *testing.T) {
	local.Reset()
	xs := []local.Opt{local.Named("x")}
	Noop(xs)
	calls := local.Record()
	if len(calls) != 1 {
		t.Fatalf("recorded %v", calls)
	}
	got := calls[0]
	if len(got) != 1 || got[0] != "x" {
		t.Fatalf("the no-op advice must not change the recorded values: %v", got)
	}
	if len(xs) != 1 || xs[0].Name != "x" {
		t.Fatalf("the no-op advice must not change the caller's slice: %v", xs)
	}
}

// TestControl is the positive control: the module's weaving is live, so the
// local.Sink call in Control gets its injected value.
func TestControl(t *testing.T) {
	local.Reset()
	Control([]local.Opt{local.Named("x")})
	calls := local.Record()
	if len(calls) != 1 {
		t.Fatalf("recorded %v", calls)
	}
	got := calls[0]
	if len(got) != 2 || got[0] != "x" || got[1] != "traced" {
		t.Fatalf("recorded %v, want [x traced]", got)
	}
}
