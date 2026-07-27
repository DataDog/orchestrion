// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package pkgs

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestResolveTestVariantsRequestHash(t *testing.T) {
	first := ResolveTestVariantsRequest{
		Dir:              "/module",
		Env:              []string{"B=2", "PWD=/first", "TOOLEXEC_IMPORTPATH=first.test", envVarParentID + "=first-parent", "A=1"},
		TempDir:          "/work/tmp",
		PackageUnderTest: "example.com/model",
		SyntheticRoots:   []string{"example.com/z", "example.com/a", "example.com/z"},
	}
	second := ResolveTestVariantsRequest{
		Dir:              "/module",
		Env:              []string{"A=1", envVarParentID + "=second-parent", "TOOLEXEC_IMPORTPATH=second.test", "PWD=/second", "B=2"},
		TempDir:          "/work/tmp",
		PackageUnderTest: "example.com/model",
		SyntheticRoots:   []string{"example.com/a", "example.com/z"},
	}

	firstHash, err := first.hash()
	require.NoError(t, err)
	secondHash, err := second.hash()
	require.NoError(t, err)
	assert.Equal(t, firstHash, secondHash)
	assert.Equal(t, []string{"example.com/a", "example.com/z"}, first.SyntheticRoots)

	second.Env = append(second.Env, "VARIANT_AFFECTING=value")
	second.canonical = false
	secondHash, err = second.hash()
	require.NoError(t, err)
	assert.NotEqual(t, firstHash, secondHash)
}

func TestTestVariantOverlay(t *testing.T) {
	source, err := testVariantOverlay("model", []string{"example.com/z", "example.com/a", "example.com/z"})
	require.NoError(t, err)
	assert.Equal(t, "package model_test\n\nimport (\n\t_ \"example.com/a\"\n\t_ \"example.com/z\"\n)\n", string(source))
}
