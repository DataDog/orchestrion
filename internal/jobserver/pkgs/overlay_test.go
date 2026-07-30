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
	"runtime"
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

func TestAddTestVariantOverlays(t *testing.T) {
	dir := t.TempDir()
	config := packages.Config{Dir: dir, Env: os.Environ()}
	overlays := map[string]overlaySource{
		filepath.Join(dir, "subject", "variant_test.go"): {contents: []byte("package subject_test\n")},
		filepath.Join(dir, "bridge", "bridge.go"):        {contents: []byte("package bridge\n"), newPackage: true},
	}
	cleanup, err := addTestVariantOverlays(context.Background(), &config, overlays)
	require.NoError(t, err)
	defer cleanup()

	require.Len(t, config.BuildFlags, 1)
	manifestPath, found := strings.CutPrefix(config.BuildFlags[0], "-overlay=")
	require.True(t, found)
	data, err := os.ReadFile(manifestPath)
	require.NoError(t, err)
	var manifest overlayManifest
	require.NoError(t, json.Unmarshal(data, &manifest))
	require.Len(t, manifest.Replace, 2)
	for virtualPath, want := range overlays {
		contents, err := os.ReadFile(manifest.Replace[virtualPath])
		require.NoError(t, err)
		assert.Equal(t, want.contents, contents)
	}
}

func TestVendoredOverlayRestrictionAppliesOnlyToNewPackages(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module example.com/subject\n\ngo 1.25\n"), 0o644))
	config := packages.Config{Dir: dir, Env: append(os.Environ(), "GOWORK=off")}
	vendoredTestFile := filepath.Join(dir, "vendor", "example.com", "dependency", "variant_test.go")

	cleanup, err := addTestVariantOverlay(context.Background(), &config, vendoredTestFile, []byte("package dependency_test\n"))
	require.NoError(t, err)
	cleanup()

	config = packages.Config{Dir: dir, Env: append(os.Environ(), "GOWORK=off")}
	_, err = addTestVariantOverlays(context.Background(), &config, map[string]overlaySource{
		filepath.Join(dir, "vendor", "example.com", "dependency", "bridge", "bridge.go"): {
			contents:   []byte("package bridge\n"),
			newPackage: true,
		},
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "vendor directory")
}

func TestActiveVendorDir(t *testing.T) {
	tests := []struct {
		name   string
		goMod  string
		goWork string
		want   string
	}{
		{name: "module", goMod: filepath.Join("workspace", "module", "go.mod"), want: filepath.Join("workspace", "module", "vendor")},
		{name: "workspace", goMod: filepath.Join("workspace", "module", "go.mod"), goWork: filepath.Join("workspace", "go.work"), want: filepath.Join("workspace", "vendor")},
		{name: "workspace off", goMod: filepath.Join("workspace", "module", "go.mod"), goWork: "off", want: filepath.Join("workspace", "module", "vendor")},
		{name: "no module"},
		{name: "no module sentinel", goMod: os.DevNull},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, activeVendorDir(tt.goMod, tt.goWork))
		})
	}
}

func TestUnrelatedVendorAncestorIsAllowed(t *testing.T) {
	project := filepath.Join(t.TempDir(), "vendor", "project")
	require.NoError(t, os.MkdirAll(project, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(project, "go.mod"), []byte("module example.com/project\n\ngo 1.25\n"), 0o644))
	config := packages.Config{Dir: project, Env: append(os.Environ(), "GOWORK=off")}
	cleanup, err := addTestVariantOverlays(context.Background(), &config, map[string]overlaySource{
		filepath.Join(project, "bridge", "bridge.go"): {contents: []byte("package bridge\n"), newPackage: true},
	})
	require.NoError(t, err)
	cleanup()
}

func TestUnsupportedOverlayRootPolicy(t *testing.T) {
	root := t.TempDir()
	virtualPath := filepath.Join(root, "pkg", "variant_test.go")
	tests := []struct {
		name       string
		root       unsupportedOverlayRoot
		newPackage bool
		wantError  bool
	}{
		{
			name:      "existing package beneath GOMODCACHE",
			root:      unsupportedOverlayRoot{name: "GOMODCACHE", path: root, why: "prohibited"},
			wantError: true,
		},
		{
			name:       "new package beneath GOMODCACHE",
			root:       unsupportedOverlayRoot{name: "GOMODCACHE", path: root, why: "prohibited"},
			newPackage: true,
			wantError:  true,
		},
		{
			name: "existing package beneath GOROOT",
			root: unsupportedOverlayRoot{name: "GOROOT", path: root, why: "prohibited", newPackageOnly: true},
		},
		{
			name:       "new package beneath GOROOT",
			root:       unsupportedOverlayRoot{name: "GOROOT", path: root, why: "prohibited", newPackageOnly: true},
			newPackage: true,
			wantError:  true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := rejectUnsupportedOverlayPath([]unsupportedOverlayRoot{tt.root}, virtualPath, tt.newPackage)
			if tt.wantError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestAddTestVariantOverlaysRejectNormalizedDuplicatePaths(t *testing.T) {
	dir := t.TempDir()
	config := packages.Config{Dir: dir, Env: os.Environ()}
	_, err := addTestVariantOverlays(context.Background(), &config, map[string]overlaySource{
		filepath.Join("pkg", "variant_test.go"):      {contents: []byte("package pkg_test\n")},
		filepath.Join(dir, "pkg", "variant_test.go"): {contents: []byte("package pkg_test\n")},
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "duplicate path")
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

	realDir := filepath.Join(root, "real")
	require.NoError(t, os.Mkdir(realDir, 0o755))
	linkDir := filepath.Join(root, "link")
	if err := os.Symlink(realDir, linkDir); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	inside, err = pathWithin(realDir, filepath.Join(linkDir, "missing", "file.go"))
	require.NoError(t, err)
	assert.True(t, inside)
}

func TestPathWithinDifferentWindowsVolumes(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("Windows volume semantics")
	}
	inside, err := pathWithin(`C:\\module`, `D:\\module\\file.go`)
	require.NoError(t, err)
	assert.False(t, inside)
}
