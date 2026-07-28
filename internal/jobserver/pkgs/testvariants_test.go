// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package pkgs

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/tools/go/packages"
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

	marked := variant
	marked.Env = append(append([]string(nil), variant.Env...), envVarResolvingTestVariants+"=1")
	marked.canonical = false
	markedHash, err := marked.hash()
	require.NoError(t, err)
	assert.Equal(t, variantHash, markedHash)

	variant.Env = append(variant.Env, "VARIANT_AFFECTING=value")
	variant.canonical = false
	changedHash, err := variant.hash()
	require.NoError(t, err)
	assert.NotEqual(t, variantHash, changedHash)
}

func TestCollectTestVariantClosureRequiresExports(t *testing.T) {
	const forTest = "example.com/subject"
	root := &packages.Package{
		ID:         "example.com/root [example.com/subject.test]",
		PkgPath:    "example.com/root",
		ForTest:    forTest,
		ExportFile: "/root.a",
		Imports: map[string]*packages.Package{
			"example.com/middle": {
				ID:      "example.com/middle [example.com/subject.test]",
				PkgPath: "example.com/middle",
				ForTest: forTest,
			},
		},
	}
	err := collectTestVariantClosure(make(ResolveResponse), root, forTest, make(map[string]bool))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "example.com/middle")
}

func TestTestVariantOverlay(t *testing.T) {
	source, err := testVariantOverlay("subject", "example.com/linkdep")
	require.NoError(t, err)
	assert.Equal(t, "package subject_test\n\nimport _ \"example.com/linkdep\"\n", string(source))
}
