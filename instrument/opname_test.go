// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package instrument

import "testing"

func TestGetOpName(t *testing.T) {
	for _, tt := range []struct {
		metadata []any
		opname   string
	}{
		{
			metadata: []any{"foo", "bar", "verb", "just-verb"},
			opname:   "just-verb",
		},
		{
			metadata: []any{"foo", "bar", "function-name", "just-function-name"},
			opname:   "just-function-name",
		},
		{
			metadata: []any{"foo", "bar", "verb", "verb-function-name", "function-name", "THIS IS WRONG"},
			opname:   "verb-function-name",
		},
		{
			// Checking different order
			metadata: []any{"foo", "bar", "function-name", "THIS IS WRONG", "verb", "verb-function-name"},
			opname:   "verb-function-name",
		},
	} {
		t.Run(tt.opname, func(t *testing.T) {
			n := getOpName(tt.metadata...)
			if n != tt.opname {
				t.Errorf("Expected %s, but got %s\n", tt.opname, n)
			}
		})
	}
}
