// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package may

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestFileContains(t *testing.T) {
	const content = "package example\n\n//orchestrion:enabled\nvar enabled bool\n"
	ctx := &FileContext{FileContent: []byte(content)}

	for needle, expected := range map[string]MatchType{
		"":                      Match,
		"//orchestrion:enabled": Match,
		content:                 Match,
		"//orchestrion:version": NeverMatch,
		content + " ":           NeverMatch,
	} {
		assert.Equal(t, expected, ctx.FileContains(needle), "lookup %q", needle)
	}
}
