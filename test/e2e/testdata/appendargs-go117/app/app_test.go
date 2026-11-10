// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package app

import (
	"testing"

	"example.com/appendargs-go117/local"
)

func TestWoven(t *testing.T) {
	local.Reset()
	Woven([]local.Opt{local.Named("x")})
	calls := local.Record()
	if len(calls) != 1 {
		t.Fatalf("recorded %v", calls)
	}
	got := calls[0]
	if len(got) != 2 || got[0] != "x" || got[1] != "traced" {
		t.Fatalf("recorded %v, want [x traced]", got)
	}
}
