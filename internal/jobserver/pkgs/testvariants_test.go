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

func TestResolveRequestTestVariantHash(t *testing.T) {
	ordinary := ResolveRequest{
		Dir:     "/module",
		Env:     []string{"B=2", "PWD=/first", "TOOLEXEC_IMPORTPATH=first.test", envVarParentID + "=first-parent", "A=1"},
		Pattern: "example.com/linkdep",
		TempDir: "/work/tmp",
	}
	variant := ResolveRequest{
		Dir:            "/module",
		Env:            []string{"A=1", envVarParentID + "=second-parent", "TOOLEXEC_IMPORTPATH=second.test", "PWD=/second", "B=2"},
		Pattern:        "example.com/linkdep",
		TempDir:        "/work/tmp",
		TestVariantFor: "example.com/subject",
	}

	ordinaryHash, err := ordinary.hash()
	require.NoError(t, err)
	variantHash, err := variant.hash()
	require.NoError(t, err)
	assert.NotEqual(t, ordinaryHash, variantHash)

	variant.Env = append(variant.Env, "VARIANT_AFFECTING=value")
	variant.canonical = false
	changedHash, err := variant.hash()
	require.NoError(t, err)
	assert.NotEqual(t, variantHash, changedHash)
}

func TestTestVariantOverlay(t *testing.T) {
	source, err := testVariantOverlay("subject", "example.com/linkdep")
	require.NoError(t, err)
	assert.Equal(t, "package subject_test\n\nimport _ \"example.com/linkdep\"\n", string(source))
}
