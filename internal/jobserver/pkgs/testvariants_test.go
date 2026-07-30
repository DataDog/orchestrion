// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package pkgs

import (
	"crypto/sha256"
	"path/filepath"
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

func TestPackageSourceDirUsesOtherFiles(t *testing.T) {
	pkg := &packages.Package{OtherFiles: []string{filepath.Join("module", "subject", "subject.swig")}}
	assert.Equal(t, filepath.Join("module", "subject"), packageSourceDir(pkg))
}

func TestInternalImportBridge(t *testing.T) {
	rootDir := filepath.Join(t.TempDir(), "dep", "internal", "root")
	root := &packages.Package{
		PkgPath: "example.com/module/dep/internal/root",
		GoFiles: []string{filepath.Join(rootDir, "root.go")},
	}
	key := sha256.Sum256([]byte("bridge"))

	bridge, needed, err := internalImportBridge(root, "example.com/module/subject", key)
	require.NoError(t, err)
	require.True(t, needed)
	assert.Equal(t, "example.com/module/dep/orchestrion_test_variant_17f29b073143d8cd", bridge.importPath)
	assert.Equal(t, filepath.Join(filepath.Dir(filepath.Dir(rootDir)), "orchestrion_test_variant_17f29b073143d8cd", "bridge.go"), bridge.virtualPath)
	assert.Equal(t, "package orchestrion_test_variant_bridge\n\nimport _ \"example.com/module/dep/internal/root\"\n", string(bridge.source))

	_, needed, err = internalImportBridge(root, "example.com/module/dep/subject", key)
	require.NoError(t, err)
	assert.False(t, needed)

	_, needed, err = internalImportBridge(root, "example.com/module/depextra/subject", key)
	require.NoError(t, err)
	assert.True(t, needed)
}

func TestInternalImportBridgeRejectsNestedInternalRoot(t *testing.T) {
	root := &packages.Package{
		PkgPath: "example.com/module/internal/dep/internal/root",
		GoFiles: []string{filepath.Join(t.TempDir(), "internal", "dep", "internal", "root", "root.go")},
	}
	_, _, err := internalImportBridge(root, "example.net/subject", sha256.Sum256([]byte("bridge")))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "nested internal")
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
