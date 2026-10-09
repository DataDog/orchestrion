// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package may

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFileContains(t *testing.T) {
	const content = "package example\n\n//orchestrion:enabled\nvar enabled bool\n"

	lookups := map[string]MatchType{
		"":                      NeverMatch,
		"package":               Match,
		"//orchestrion:enabled": Match,
		"enabled bool\n":        Match,
		content:                 Match,
		"//orchestrion:version": NeverMatch,
		content + " ":           NeverMatch,
	}

	// Enough passes for lookups to be answered both by scanning the content, then by the index.
	ctx := &FileContext{FileContent: []byte(content)}
	for pass := 0; pass <= maxLinearLookups/len(lookups)+1; pass++ {
		for needle, expected := range lookups {
			assert.Equal(t, expected, ctx.FileContains(needle), "pass %d, lookup %q", pass, needle)
		}
	}
	require.NotNil(t, ctx.index, "lookups were never answered by the index")
}
