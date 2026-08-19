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

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/tools/go/packages"
)

func TestReverseVariantEnvironment(t *testing.T) {
	t.Setenv(envVarReverseVariant, "")
	t.Setenv(envVarReverseVariantFlavor, "")

	flavor, packageFiles, active, err := ReverseVariantEnvironment()
	require.NoError(t, err)
	assert.False(t, active)
	assert.Empty(t, flavor)
	assert.Nil(t, packageFiles)

	path := filepath.Join(t.TempDir(), "environment.json")
	t.Setenv(envVarReverseVariant, path)
	_, _, active, err = ReverseVariantEnvironment()
	assert.True(t, active)
	require.ErrorContains(t, err, "reading reverse test variant environment")

	tests := []struct {
		name          string
		contents      string
		processFlavor string
		wantError     string
	}{
		{name: "invalid JSON", contents: "{", wantError: "parsing reverse test variant environment"},
		{name: "missing flavor", contents: `{"packageFiles":{"example.com/root":"/root.a"}}`, wantError: "has no flavor"},
		{name: "flavor mismatch", contents: `{"flavor":"expected","packageFiles":{"example.com/root":"/root.a"}}`, processFlavor: "actual", wantError: `has flavor "expected", process declares "actual"`},
		{name: "missing package files", contents: `{"flavor":"expected"}`, processFlavor: "expected", wantError: "has no package files"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.NoError(t, os.WriteFile(path, []byte(tt.contents), 0o644))
			t.Setenv(envVarReverseVariantFlavor, tt.processFlavor)
			_, _, active, err := ReverseVariantEnvironment()
			assert.True(t, active)
			require.ErrorContains(t, err, tt.wantError)
		})
	}

	require.NoError(t, os.WriteFile(path, []byte(`{"flavor":"expected","packageFiles":{"example.com/root":"/root.a"}}`), 0o644))
	t.Setenv(envVarReverseVariantFlavor, "expected")
	flavor, packageFiles, active, err = ReverseVariantEnvironment()
	require.NoError(t, err)
	assert.True(t, active)
	assert.Equal(t, "expected", flavor)
	assert.Equal(t, map[string]string{"example.com/root": "/root.a"}, packageFiles)
}

func TestAddReverseVariantEnvironment(t *testing.T) {
	config := packages.Config{Env: []string{"EXISTING=value"}}
	packageFiles := map[string]string{"example.com/root": "/root.a"}
	cleanup, err := addReverseVariantEnvironment(&config, t.TempDir(), "flavor", packageFiles)
	require.NoError(t, err)
	require.Len(t, config.Env, 3)
	assert.Equal(t, envVarReverseVariantFlavor+"=flavor", config.Env[2])

	path, found := strings.CutPrefix(config.Env[1], envVarReverseVariant+"=")
	require.True(t, found)
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	var environment reverseVariantEnvironment
	require.NoError(t, json.Unmarshal(data, &environment))
	assert.Equal(t, reverseVariantEnvironment{Flavor: "flavor", PackageFiles: packageFiles}, environment)

	cleanup()
	_, err = os.Stat(filepath.Dir(path))
	require.ErrorIs(t, err, os.ErrNotExist)

	invalidParent := filepath.Join(t.TempDir(), "file")
	require.NoError(t, os.WriteFile(invalidParent, nil, 0o644))
	_, err = addReverseVariantEnvironment(&packages.Config{}, invalidParent, "flavor", packageFiles)
	require.ErrorContains(t, err, "creating reverse test variant environment")
}

func TestCollectReverseVariantClosure(t *testing.T) {
	const target = "example.com/subject"
	subject := &packages.Package{ID: target, PkgPath: target, ExportFile: "/subject.a"}
	unsafe := &packages.Package{ID: "unsafe", PkgPath: "unsafe"}
	middle := &packages.Package{
		ID:         "example.com/middle",
		PkgPath:    "example.com/middle",
		ExportFile: "/middle.a",
		Imports:    map[string]*packages.Package{target: subject, "unsafe": unsafe},
	}
	root := &packages.Package{
		ID:         "example.com/root",
		PkgPath:    "example.com/root",
		ExportFile: "/root.a",
		Imports:    map[string]*packages.Package{middle.PkgPath: middle},
	}
	resp := make(ResolveResponse)
	require.NoError(t, collectReverseVariantClosure(context.Background(), resp, root, target, make(map[string]bool)))
	assert.Equal(t, ResolvedArchive{ExportFile: "/root.a", ForTest: target}, resp[root.PkgPath])
	assert.Equal(t, ResolvedArchive{ExportFile: "/middle.a", ForTest: target}, resp[middle.PkgPath])
	assert.NotContains(t, resp, target)
	assert.NotContains(t, resp, "unsafe")

	missing := &packages.Package{ID: "example.com/missing", PkgPath: "example.com/missing"}
	err := collectReverseVariantClosure(context.Background(), make(ResolveResponse), missing, target, make(map[string]bool))
	require.ErrorContains(t, err, "did not produce an export archive")

	visited := map[string]bool{missing.ID: true}
	shortCircuited := make(ResolveResponse)
	require.NoError(t, collectReverseVariantClosure(context.Background(), shortCircuited, missing, target, visited))
	assert.Empty(t, shortCircuited)

	shortCircuited = make(ResolveResponse)
	require.NoError(t, collectReverseVariantClosure(context.Background(), shortCircuited, nil, target, make(map[string]bool)))
	assert.Empty(t, shortCircuited)
}

func TestResolveReverseTestVariantRejectsIncompleteRequest(t *testing.T) {
	_, err := (&service{}).resolveReverseTestVariant(context.Background(), &ResolveRequest{Pattern: "example.com/root"}, zerolog.Nop())
	require.ErrorContains(t, err, "incomplete reverse test variant request")
}

func TestBuildReverseTestVariantStandaloneRoot(t *testing.T) {
	const (
		module = "example.com/reverse"
		target = module + "/target"
		root   = module + "/root"
	)
	dir := t.TempDir()
	writeFile := func(name string, contents string) {
		t.Helper()
		path := filepath.Join(dir, filepath.FromSlash(name))
		require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
		require.NoError(t, os.WriteFile(path, []byte(contents), 0o644))
	}
	writeFile("go.mod", "module "+module+"\n\ngo 1.25.0\n")
	writeFile("target/target.go", "package target\n\nconst Value = 42\n")
	writeFile("target/target_test.go", "package target_test\n\nimport (\"testing\"; \"example.com/reverse/target\")\n\nfunc TestValue(t *testing.T) { if target.Value != 42 { t.Fail() } }\n")
	writeFile("root/root.go", "package root\n\nimport \"example.com/reverse/target\"\n\nconst Value = target.Value\n")

	config := packages.Config{
		Dir: dir,
		Env: append(os.Environ(), "GOWORK=off", "GOFLAGS="),
		Mode: packages.NeedName | packages.NeedImports | packages.NeedDeps |
			packages.NeedExportFile | packages.NeedForTest,
	}
	loaded, err := packages.Load(&config, target)
	require.NoError(t, err)
	require.NoError(t, packageErrors(loaded))
	selected := findPackage(collectPackages(loaded), target)
	require.NotNil(t, selected)
	require.NotEmpty(t, selected.ExportFile)

	result, err := buildReverseTestVariant(&ResolveRequest{
		Pattern:             root,
		TestVariantFor:      target,
		AuthoritativeTarget: selected.ExportFile,
		TempDir:             t.TempDir(),
	}, config)
	require.NoError(t, err)
	assert.Equal(t, target, result.response[root].ForTest)
	assert.NotEmpty(t, result.response[root].ExportFile)
	assert.NotContains(t, result.response, target)
}
