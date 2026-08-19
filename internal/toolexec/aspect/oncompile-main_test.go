// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package aspect

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/DataDog/orchestrion/internal/jobserver/pkgs"
	"github.com/DataDog/orchestrion/internal/toolexec/aspect/linkdeps"
	"github.com/DataDog/orchestrion/internal/toolexec/importcfg"
	"github.com/blakesmith/ar"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestResolveReverseTestVariantsSkipsProvenanceWithoutTargetImporter(t *testing.T) {
	const target = "example.com/subject"
	for _, tt := range []struct {
		name              string
		withTargetPackage bool
	}{
		{name: "package under test", withTargetPackage: true},
		{name: "external tests only"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			reg := importcfg.ImportConfig{PackageFile: map[string]string{
				"example.com/parent": writeLinkDepsArchive(t, "parent.a", "example.com/other", linkdeps.ImportDependency),
			}}
			if tt.withTargetPackage {
				reg.PackageFile[target] = writeLinkDepsArchive(t, "subject.a", "", linkdeps.RelocationDependency)
			}

			packageFiles, roots, err := resolveReverseTestVariants(
				context.Background(),
				&reg,
				target,
				t.TempDir(),
				func() (pkgs.ResolvedArchive, error) {
					t.Fatal("unexpected target provenance resolution")
					return pkgs.ResolvedArchive{}, nil
				},
			)
			require.NoError(t, err)
			assert.Empty(t, packageFiles)
			assert.Empty(t, roots)
		})
	}
}

func TestResolveReverseTestVariantsValidateProvenance(t *testing.T) {
	const target = "example.com/subject"
	parent := writeLinkDepsArchive(t, "parent.a", target, linkdeps.ImportDependency)

	t.Run("ordinary target", func(t *testing.T) {
		reg := importcfg.ImportConfig{PackageFile: map[string]string{
			"example.com/parent": parent,
			target:               writeLinkDepsArchive(t, "subject.a", "", linkdeps.RelocationDependency),
		}}
		packageFiles, roots, err := resolveReverseTestVariants(
			context.Background(),
			&reg,
			target,
			t.TempDir(),
			func() (pkgs.ResolvedArchive, error) { return pkgs.ResolvedArchive{}, nil },
		)
		require.NoError(t, err)
		assert.Empty(t, packageFiles)
		assert.Empty(t, roots)
	})

	t.Run("missing authoritative target", func(t *testing.T) {
		reg := importcfg.ImportConfig{PackageFile: map[string]string{"example.com/parent": parent}}
		_, _, err := resolveReverseTestVariants(
			context.Background(),
			&reg,
			target,
			t.TempDir(),
			func() (pkgs.ResolvedArchive, error) { return pkgs.ResolvedArchive{ForTest: target}, nil },
		)
		require.ErrorContains(t, err, "missing the authoritative package-under-test archive")
	})

	t.Run("provenance error", func(t *testing.T) {
		reg := importcfg.ImportConfig{PackageFile: map[string]string{"example.com/parent": parent}}
		_, _, err := resolveReverseTestVariants(
			context.Background(),
			&reg,
			target,
			t.TempDir(),
			func() (pkgs.ResolvedArchive, error) { return pkgs.ResolvedArchive{}, assert.AnError },
		)
		require.ErrorIs(t, err, assert.AnError)
	})
}

func TestInitialLinkDependenciesRetainParent(t *testing.T) {
	archive := writeLinkDepsArchive(t, "parent.a", "example.com/dependency", linkdeps.ImportDependency)
	reg := importcfg.ImportConfig{PackageFile: map[string]string{"example.com/parent": archive}}

	deps, err := initialLinkDependencies(context.Background(), &reg, "example.com/subject", func() (pkgs.ResolvedArchive, error) {
		t.Fatal("unexpected target provenance resolution")
		return pkgs.ResolvedArchive{}, nil
	}, nil)
	require.NoError(t, err)
	require.Len(t, deps, 1)
	assert.Equal(t, "example.com/dependency", deps[0].path)
	assert.Equal(t, "example.com/parent", deps[0].parent)
	assert.Equal(t, linkdeps.ImportDependency, deps[0].kind)
}

func TestInitialLinkDependenciesPreserveTestTargetImportParent(t *testing.T) {
	archive := writeLinkDepsArchive(t, "subject.a", "example.com/dependency", linkdeps.ImportDependency)
	reg := importcfg.ImportConfig{PackageFile: map[string]string{"example.com/subject": archive}}

	deps, err := initialLinkDependencies(context.Background(), &reg, "example.com/subject", func() (pkgs.ResolvedArchive, error) {
		t.Fatal("unexpected target provenance resolution")
		return pkgs.ResolvedArchive{}, nil
	}, nil)
	require.NoError(t, err)
	require.Len(t, deps, 1)
	assert.Equal(t, "example.com/subject", deps[0].parent)
}

func TestInitialLinkDependenciesValidateSatisfiedTargetImport(t *testing.T) {
	const target = "example.com/subject"
	root := writeLinkDepsArchive(t, "root.a", target, linkdeps.ImportDependency)
	targetArchive := writeLinkDepsArchive(t, "subject.a", "", linkdeps.RelocationDependency)
	reg := importcfg.ImportConfig{PackageFile: map[string]string{
		"example.com/root": root,
		target:             targetArchive,
	}}

	t.Run("same-package tests", func(t *testing.T) {
		_, err := initialLinkDependencies(context.Background(), &reg, target, func() (pkgs.ResolvedArchive, error) {
			return pkgs.ResolvedArchive{ForTest: target}, nil
		}, nil)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "an archive in that synthetic dependency closure was compiled without this edge")
	})

	t.Run("external tests only", func(t *testing.T) {
		deps, err := initialLinkDependencies(context.Background(), &reg, target, func() (pkgs.ResolvedArchive, error) {
			return pkgs.ResolvedArchive{}, nil
		}, nil)
		require.NoError(t, err)
		assert.Empty(t, deps)
	})

	t.Run("relocation does not resolve provenance", func(t *testing.T) {
		relocationRoot := writeLinkDepsArchive(t, "relocation-root.a", target, linkdeps.RelocationDependency)
		relocationReg := importcfg.ImportConfig{PackageFile: map[string]string{
			"example.com/root": relocationRoot,
			target:             targetArchive,
		}}
		deps, err := initialLinkDependencies(context.Background(), &relocationReg, target, func() (pkgs.ResolvedArchive, error) {
			t.Fatal("unexpected target provenance resolution")
			return pkgs.ResolvedArchive{}, nil
		}, nil)
		require.NoError(t, err)
		assert.Empty(t, deps)
	})
}

func TestInitialLinkDependenciesSkipRebuiltTargetImport(t *testing.T) {
	const target = "example.com/subject"
	archive := writeLinkDepsArchive(t, "root.a", target, linkdeps.ImportDependency)
	reg := importcfg.ImportConfig{PackageFile: map[string]string{
		"example.com/root": archive,
		target:             writeLinkDepsArchive(t, "subject.a", "", linkdeps.RelocationDependency),
	}}

	deps, err := initialLinkDependencies(context.Background(), &reg, target, func() (pkgs.ResolvedArchive, error) {
		t.Fatal("unexpected target provenance resolution")
		return pkgs.ResolvedArchive{}, nil
	}, map[string]string{"example.com/root": archive})
	require.NoError(t, err)
	assert.Empty(t, deps)
}

func TestResolvedClosureImportsTarget(t *testing.T) {
	const target = "example.com/subject"

	importsTarget, err := resolvedClosureImportsTarget(context.Background(), nil, "")
	require.NoError(t, err)
	assert.False(t, importsTarget)

	importArchive := writeLinkDepsArchive(t, "import.a", target, linkdeps.ImportDependency)
	importsTarget, err = resolvedClosureImportsTarget(context.Background(), pkgs.ResolveResponse{
		"example.com/root": {ExportFile: importArchive},
	}, target)
	require.NoError(t, err)
	assert.True(t, importsTarget)

	relocationArchive := writeLinkDepsArchive(t, "relocation.a", target, linkdeps.RelocationDependency)
	importsTarget, err = resolvedClosureImportsTarget(context.Background(), pkgs.ResolveResponse{
		"example.com/root": {ExportFile: relocationArchive},
	}, target)
	require.NoError(t, err)
	assert.False(t, importsTarget)

	importsTarget, err = resolvedClosureImportsTarget(context.Background(), pkgs.ResolveResponse{
		"example.com/root": {ExportFile: importArchive, ForTest: target},
	}, target)
	require.NoError(t, err)
	assert.False(t, importsTarget)

	invalid := filepath.Join(t.TempDir(), "invalid.a")
	file, err := os.Create(invalid)
	require.NoError(t, err)
	writer := ar.NewWriter(file)
	require.NoError(t, writer.WriteGlobalHeader())
	contents := []byte("invalid")
	require.NoError(t, writer.WriteHeader(&ar.Header{Name: linkdeps.Filename, Mode: 0o644, Size: int64(len(contents))}))
	_, err = writer.Write(contents)
	require.NoError(t, err)
	require.NoError(t, file.Close())
	_, err = resolvedClosureImportsTarget(context.Background(), pkgs.ResolveResponse{
		"example.com/root": {ExportFile: invalid},
	}, target)
	require.ErrorContains(t, err, "reading link.deps")
}

func TestWriteMainImportConfig(t *testing.T) {
	filename := filepath.Join(t.TempDir(), "importcfg")
	require.NoError(t, os.WriteFile(filename, []byte("original\n"), 0o644))
	reg := importcfg.ImportConfig{PackageFile: map[string]string{"example.com/root": "/root.a"}}

	require.NoError(t, writeMainImportConfig(zerolog.Nop(), &reg, filename))
	backup, err := os.ReadFile(filename + ".original")
	require.NoError(t, err)
	assert.Equal(t, "original\n", string(backup))
	updated, err := os.ReadFile(filename)
	require.NoError(t, err)
	assert.Contains(t, string(updated), "packagefile example.com/root=/root.a")

	reg.PackageFile["example.com/second"] = "/second.a"
	require.NoError(t, writeMainImportConfig(zerolog.Nop(), &reg, filename))
	backup, err = os.ReadFile(filename + ".original")
	require.NoError(t, err)
	assert.Equal(t, "original\n", string(backup))
}

func TestStrongestPendingLinkDep(t *testing.T) {
	tests := []struct {
		name  string
		left  pendingLinkDep
		right pendingLinkDep
		want  pendingLinkDep
	}{
		{
			name:  "import wins over relocation",
			left:  pendingLinkDep{path: "dep", parent: "relocation-parent", kind: linkdeps.RelocationDependency},
			right: pendingLinkDep{path: "dep", parent: "import-parent", kind: linkdeps.ImportDependency},
			want:  pendingLinkDep{path: "dep", parent: "import-parent", kind: linkdeps.ImportDependency},
		},
		{
			name:  "non-target parent wins for equal kinds",
			left:  pendingLinkDep{path: "dep", kind: linkdeps.ImportDependency},
			right: pendingLinkDep{path: "dep", parent: "parent", kind: linkdeps.ImportDependency},
			want:  pendingLinkDep{path: "dep", parent: "parent", kind: linkdeps.ImportDependency},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, strongestPendingLinkDep(tt.left, tt.right))
		})
	}
}

func writeLinkDepsArchive(t *testing.T, name string, dependency string, kind linkdeps.DependencyKind) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	file, err := os.Create(path)
	require.NoError(t, err)
	defer file.Close()

	writer := ar.NewWriter(file)
	require.NoError(t, writer.WriteGlobalHeader())
	var metadata linkdeps.LinkDeps
	if dependency != "" {
		metadata.Add(dependency, kind)
	}
	var contents bytes.Buffer
	require.NoError(t, metadata.Write(&contents))
	require.NoError(t, writer.WriteHeader(&ar.Header{Name: linkdeps.Filename, Mode: 0o644, Size: int64(contents.Len())}))
	_, err = writer.Write(contents.Bytes())
	require.NoError(t, err)
	return path
}
