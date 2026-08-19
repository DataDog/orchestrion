// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package aspect

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/DataDog/orchestrion/internal/jobserver/pkgs"
	"github.com/DataDog/orchestrion/internal/toolexec/importcfg"
	"github.com/DataDog/orchestrion/internal/toolexec/proxy"
	"github.com/blakesmith/ar"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRefreshTestMainPackageFiles(t *testing.T) {
	t.Run("empty", func(t *testing.T) {
		require.NoError(t, refreshTestMainPackageFiles(
			context.Background(),
			&proxy.TestMainInfo{},
			&importcfg.ImportConfig{},
			t.TempDir(),
			unexpectedReversePackageFilesResolver(t),
		))
	})

	t.Run("fresh", func(t *testing.T) {
		archive := writeLinkDepsArchive(t, "root.a", "", 0)
		info := proxy.TestMainInfo{
			Target:              "example.com/subject",
			PackageFiles:        map[string]string{"example.com/root": archive},
			PackageFingerprints: map[string]string{"example.com/root": "none"},
		}
		require.NoError(t, refreshTestMainPackageFiles(
			context.Background(),
			&info,
			&importcfg.ImportConfig{},
			t.TempDir(),
			unexpectedReversePackageFilesResolver(t),
		))
		assert.Equal(t, archive, info.PackageFiles["example.com/root"])
	})

	t.Run("stale without roots", func(t *testing.T) {
		info := proxy.TestMainInfo{
			Target:              "example.com/subject",
			PackageFiles:        map[string]string{"example.com/root": filepath.Join(t.TempDir(), "missing.a")},
			PackageFingerprints: map[string]string{"example.com/root": "fingerprint"},
		}
		err := refreshTestMainPackageFiles(
			context.Background(),
			&info,
			&importcfg.ImportConfig{},
			t.TempDir(),
			unexpectedReversePackageFilesResolver(t),
		)
		require.ErrorContains(t, err, "has no reconstruction roots")
	})

	t.Run("missing authoritative target", func(t *testing.T) {
		info := proxy.TestMainInfo{
			Target:              "example.com/subject",
			PackageFiles:        map[string]string{"example.com/root": filepath.Join(t.TempDir(), "missing.a")},
			PackageFingerprints: map[string]string{"example.com/root": "fingerprint"},
			ReverseRoots:        []string{"example.com/root"},
		}
		err := refreshTestMainPackageFiles(
			context.Background(),
			&info,
			&importcfg.ImportConfig{PackageFile: make(map[string]string)},
			t.TempDir(),
			unexpectedReversePackageFilesResolver(t),
		)
		require.ErrorContains(t, err, "missing the authoritative package-under-test archive")
	})

	t.Run("reconstructs stale metadata", func(t *testing.T) {
		const (
			target = "example.com/subject"
			root   = "example.com/root"
		)
		authoritative := writeLinkDepsArchive(t, "subject.a", "", 0)
		rebuilt := writeFingerprintedArchive(t, "rebuilt-root.a", []byte{0x01, 0x23, 0x45, 0x67, 0x89, 0xab, 0xcd, 0xef})
		workDir := t.TempDir()
		info := proxy.TestMainInfo{
			Target:              target,
			PackageFiles:        map[string]string{root: filepath.Join(t.TempDir(), "missing.a")},
			PackageFingerprints: map[string]string{root: "0123456789abcdef"},
			ReverseRoots:        []string{root},
		}
		calls := 0
		resolve := func(
			_ context.Context,
			importPath string,
			testVariantFor string,
			authoritativeTarget string,
			gotWorkDir string,
		) (pkgs.ResolveResponse, error) {
			calls++
			assert.Equal(t, root, importPath)
			assert.Equal(t, target, testVariantFor)
			assert.Equal(t, authoritative, authoritativeTarget)
			assert.Equal(t, workDir, gotWorkDir)
			return pkgs.ResolveResponse{
				root: {ExportFile: rebuilt, ForTest: target},
			}, nil
		}

		require.NoError(t, refreshTestMainPackageFiles(
			context.Background(),
			&info,
			&importcfg.ImportConfig{PackageFile: map[string]string{target: authoritative}},
			workDir,
			resolve,
		))
		assert.Equal(t, 1, calls)
		assert.Equal(t, map[string]string{root: rebuilt}, info.PackageFiles)
	})

	t.Run("rejects mismatched reconstruction", func(t *testing.T) {
		const (
			target = "example.com/subject"
			root   = "example.com/root"
		)
		authoritative := writeLinkDepsArchive(t, "subject.a", "", 0)
		rebuilt := writeFingerprintedArchive(t, "rebuilt-root.a", []byte{0xfe, 0xdc, 0xba, 0x98, 0x76, 0x54, 0x32, 0x10})
		missing := filepath.Join(t.TempDir(), "missing.a")
		info := proxy.TestMainInfo{
			Target:              target,
			PackageFiles:        map[string]string{root: missing},
			PackageFingerprints: map[string]string{root: "0123456789abcdef"},
			ReverseRoots:        []string{root},
		}
		resolve := func(context.Context, string, string, string, string) (pkgs.ResolveResponse, error) {
			return pkgs.ResolveResponse{
				root: {ExportFile: rebuilt, ForTest: target},
			}, nil
		}

		err := refreshTestMainPackageFiles(
			context.Background(),
			&info,
			&importcfg.ImportConfig{PackageFile: map[string]string{target: authoritative}},
			t.TempDir(),
			resolve,
		)
		require.ErrorContains(t, err, "has fingerprint fedcba9876543210, cached test main expects 0123456789abcdef")
		assert.Equal(t, map[string]string{root: missing}, info.PackageFiles)
	})

	t.Run("reports reconstruction errors", func(t *testing.T) {
		const (
			target = "example.com/subject"
			root   = "example.com/root"
		)
		authoritative := writeLinkDepsArchive(t, "subject.a", "", 0)
		missing := filepath.Join(t.TempDir(), "missing.a")
		info := proxy.TestMainInfo{
			Target:              target,
			PackageFiles:        map[string]string{root: missing},
			PackageFingerprints: map[string]string{root: "none"},
			ReverseRoots:        []string{root},
		}
		resolve := func(context.Context, string, string, string, string) (pkgs.ResolveResponse, error) {
			return nil, assert.AnError
		}

		err := refreshTestMainPackageFiles(
			context.Background(),
			&info,
			&importcfg.ImportConfig{PackageFile: map[string]string{target: authoritative}},
			t.TempDir(),
			resolve,
		)
		require.ErrorIs(t, err, assert.AnError)
		require.ErrorContains(t, err, "reconstructing cached reverse test variant")
		assert.Equal(t, map[string]string{root: missing}, info.PackageFiles)
	})

	t.Run("rejects omitted reconstruction", func(t *testing.T) {
		const (
			target = "example.com/subject"
			root   = "example.com/root"
		)
		authoritative := writeLinkDepsArchive(t, "subject.a", "", 0)
		missing := filepath.Join(t.TempDir(), "missing.a")
		info := proxy.TestMainInfo{
			Target:              target,
			PackageFiles:        map[string]string{root: missing},
			PackageFingerprints: map[string]string{root: "none"},
			ReverseRoots:        []string{root},
		}
		resolve := func(context.Context, string, string, string, string) (pkgs.ResolveResponse, error) {
			return make(pkgs.ResolveResponse), nil
		}

		err := refreshTestMainPackageFiles(
			context.Background(),
			&info,
			&importcfg.ImportConfig{PackageFile: map[string]string{target: authoritative}},
			t.TempDir(),
			resolve,
		)
		require.ErrorContains(t, err, "omitted \"example.com/root\"")
		assert.Equal(t, map[string]string{root: missing}, info.PackageFiles)
	})

	t.Run("rejects unreadable reconstruction", func(t *testing.T) {
		const (
			target = "example.com/subject"
			root   = "example.com/root"
		)
		authoritative := writeLinkDepsArchive(t, "subject.a", "", 0)
		missing := filepath.Join(t.TempDir(), "missing.a")
		info := proxy.TestMainInfo{
			Target:              target,
			PackageFiles:        map[string]string{root: missing},
			PackageFingerprints: map[string]string{root: "none"},
			ReverseRoots:        []string{root},
		}
		resolve := func(context.Context, string, string, string, string) (pkgs.ResolveResponse, error) {
			return pkgs.ResolveResponse{
				root: {ExportFile: filepath.Join(t.TempDir(), "missing-rebuilt.a"), ForTest: target},
			}, nil
		}

		err := refreshTestMainPackageFiles(
			context.Background(),
			&info,
			&importcfg.ImportConfig{PackageFile: map[string]string{target: authoritative}},
			t.TempDir(),
			resolve,
		)
		require.ErrorContains(t, err, "fingerprinting reconstructed reverse test variant \"example.com/root\"")
		assert.Equal(t, map[string]string{root: missing}, info.PackageFiles)
	})
}

func unexpectedReversePackageFilesResolver(t *testing.T) reversePackageFilesResolver {
	t.Helper()
	return func(context.Context, string, string, string, string) (pkgs.ResolveResponse, error) {
		t.Helper()
		assert.Fail(t, "reverse package-files resolver was called unexpectedly")
		return nil, assert.AnError
	}
}

func writeFingerprintedArchive(t *testing.T, name string, fingerprint []byte) string {
	t.Helper()
	require.Len(t, fingerprint, 8)

	path := filepath.Join(t.TempDir(), name)
	file, err := os.Create(path)
	require.NoError(t, err)
	writer := ar.NewWriter(file)
	require.NoError(t, writer.WriteGlobalHeader())
	object := append([]byte("go object header\n!\n\x00go120ld"), fingerprint...)
	require.NoError(t, writer.WriteHeader(&ar.Header{Name: "_go_.o", Mode: 0o644, Size: int64(len(object))}))
	_, err = writer.Write(object)
	require.NoError(t, err)
	require.NoError(t, file.Close())
	return path
}
