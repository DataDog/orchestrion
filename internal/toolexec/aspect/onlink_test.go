// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package aspect

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/DataDog/orchestrion/internal/toolexec/importcfg"
	"github.com/DataDog/orchestrion/internal/toolexec/proxy"
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
		))
		assert.Equal(t, archive, info.PackageFiles["example.com/root"])
	})

	t.Run("stale without roots", func(t *testing.T) {
		info := proxy.TestMainInfo{
			Target:              "example.com/subject",
			PackageFiles:        map[string]string{"example.com/root": filepath.Join(t.TempDir(), "missing.a")},
			PackageFingerprints: map[string]string{"example.com/root": "fingerprint"},
		}
		err := refreshTestMainPackageFiles(context.Background(), &info, &importcfg.ImportConfig{}, t.TempDir())
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
		)
		require.ErrorContains(t, err, "missing the authoritative package-under-test archive")
	})
}
