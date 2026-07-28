// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package pkgs

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/tools/go/packages"
)

func TestAddTestVariantOverlay(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "caller.json"), []byte(`{"Replace":{"subject.go":"replacement.go","deleted.go":""}}`), 0o644))
	config := packages.Config{
		Dir:        dir,
		Env:        os.Environ(),
		BuildFlags: []string{"-tags=test", "-overlay=caller.json"},
	}
	virtualPath := filepath.Join(dir, "zz_orchestrion_test.go")
	cleanup, err := addTestVariantOverlay(context.Background(), &config, virtualPath, []byte("package subject_test\n"))
	require.NoError(t, err)
	defer cleanup()

	require.Len(t, config.BuildFlags, 2)
	assert.Equal(t, "-tags=test", config.BuildFlags[0])
	manifestPath, found := strings.CutPrefix(config.BuildFlags[1], "-overlay=")
	require.True(t, found)
	data, err := os.ReadFile(manifestPath)
	require.NoError(t, err)
	var manifest overlayManifest
	require.NoError(t, json.Unmarshal(data, &manifest))
	assert.Equal(t, filepath.Join(dir, "replacement.go"), manifest.Replace[filepath.Join(dir, "subject.go")])
	assert.Empty(t, manifest.Replace[filepath.Join(dir, "deleted.go")])
	backing := manifest.Replace[virtualPath]
	assert.NotEmpty(t, backing)
	contents, err := os.ReadFile(backing)
	require.NoError(t, err)
	assert.Equal(t, "package subject_test\n", string(contents))
}

func TestAddTestVariantOverlayRejectsDuplicatePaths(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "caller.json"), []byte(`{"Replace":{"subject.go":"one.go","./subject.go":"two.go"}}`), 0o644))
	config := packages.Config{Dir: dir, Env: os.Environ(), BuildFlags: []string{"-overlay=caller.json"}}
	_, err := addTestVariantOverlay(context.Background(), &config, filepath.Join(dir, "zz_orchestrion_test.go"), []byte("package subject_test\n"))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "duplicate paths")
}

func TestPathWithin(t *testing.T) {
	root := t.TempDir()
	inside, err := pathWithin(root, filepath.Join(root, "module", "file.go"))
	require.NoError(t, err)
	assert.True(t, inside)
	outside, err := pathWithin(root, filepath.Join(filepath.Dir(root), "elsewhere", "file.go"))
	require.NoError(t, err)
	assert.False(t, outside)
}
