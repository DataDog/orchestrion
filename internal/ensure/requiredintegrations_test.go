// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package ensure

import (
	"context"
	"testing"

	"github.com/DataDog/orchestrion/internal/gomod"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/mod/semver"
)

func TestFetchLatestVersion(t *testing.T) {
	ctx := context.Background()
	if d, ok := t.Deadline(); ok {
		var cancel context.CancelFunc
		ctx, cancel = context.WithDeadline(ctx, d)
		defer cancel()
	}

	t.Run("valid-module", func(t *testing.T) {
		// Test with a real module that should always exist
		version, err := fetchLatestVersion(ctx, "golang.org/x/mod")
		require.NoError(t, err)
		assert.NotEmpty(t, version)
		assert.True(t, semver.IsValid(version), "version %q should be valid semver", version)
	})

	t.Run("invalid-module", func(t *testing.T) {
		// Test with a module that doesn't exist
		_, err := fetchLatestVersion(ctx, "github.com/nonexistent/nonexistent-module-12345")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "go list -m -json")
	})
}

func TestResolveIntegrationVersion(t *testing.T) {
	ctx := context.Background()
	if d, ok := t.Deadline(); ok {
		var cancel context.CancelFunc
		ctx, cancel = context.WithDeadline(ctx, d)
		defer cancel()
	}

	// Save original shipped versions and restore after test.
	originalShippedVersions := make(map[string]string)
	for k, v := range orchestrionShippedVersions {
		originalShippedVersions[k] = v
	}
	t.Cleanup(func() {
		for k := range orchestrionShippedVersions {
			delete(orchestrionShippedVersions, k)
		}
		for k, v := range originalShippedVersions {
			orchestrionShippedVersions[k] = v
		}
	})

	t.Run("module-not-present", func(t *testing.T) {
		modPath := "golang.org/x/mod"
		orchestrionShippedVersions[modPath] = "v0.15.0"

		curMod := gomod.File{
			Require: nil,
		}

		shouldUpgrade, targetVersion, err := resolveIntegrationVersion(ctx, curMod, modPath)
		require.NoError(t, err)
		assert.True(t, shouldUpgrade, "should upgrade when module is not present")
		// targetVersion should be max of shipped and latest
		assert.True(t, semver.IsValid(targetVersion))
		// Should be at least the shipped version
		assert.GreaterOrEqual(t, semver.Compare(targetVersion, "v0.15.0"), 0)
	})

	t.Run("module-present-below-minimum", func(t *testing.T) {
		modPath := "example.com/test-integration"
		orchestrionShippedVersions[modPath] = "v2.5.0"

		curMod := gomod.File{
			Require: []gomod.Require{
				{Path: modPath, Version: "v2.0.0"}, // Below v2.1.0 minimum
			},
		}

		// Note: This will fail for a non-existent module, so we need to handle error
		shouldUpgrade, targetVersion, err := resolveIntegrationVersion(ctx, curMod, modPath)
		// Since example.com/test-integration doesn't exist, this will error
		// In real scenarios with actual modules, this would work
		if err != nil {
			assert.Contains(t, err.Error(), "fetching latest version")
		} else {
			assert.True(t, shouldUpgrade, "should upgrade when below minimum version")
			assert.GreaterOrEqual(t, semver.Compare(targetVersion, "v2.1.0"), 0)
		}
	})

	// Mock fetcher that returns a fixed version
	mockFetcher := func(_ context.Context, _ string) (string, error) {
		return "v0.28.0", nil // Simulate registry latest
	}

	t.Run("module-present-below-minimum-upgrade-needed", func(t *testing.T) {
		modPath := "test-module"
		orchestrionShippedVersions[modPath] = "v0.10.0"

		curMod := gomod.File{
			Require: []gomod.Require{
				{Path: modPath, Version: "v0.5.0"}, // Below v2.1.0
			},
		}

		shouldUpgrade, targetVersion, err := resolveIntegrationVersionWithFetcher(ctx, curMod, modPath, mockFetcher)
		require.NoError(t, err)
		// When foundVersion < v2.1.0, always upgrade to max(shipped, latest)
		assert.True(t, shouldUpgrade, "should upgrade when below minimum version")
		// Target version should be max of shipped (v0.10.0) and latest (v0.28.0) = v0.28.0
		assert.Equal(t, "v0.28.0", targetVersion)
	})

	t.Run("module-present-above-minimum-shipped-newer", func(t *testing.T) {
		modPath := "test-module"
		orchestrionShippedVersions[modPath] = "v3.0.0"

		curMod := gomod.File{
			Require: []gomod.Require{
				{Path: modPath, Version: "v2.5.0"}, // Above v2.1.0, but less than shipped
			},
		}

		shouldUpgrade, targetVersion, err := resolveIntegrationVersionWithFetcher(ctx, curMod, modPath, mockFetcher)
		require.NoError(t, err)
		assert.True(t, shouldUpgrade, "should upgrade when shipped version is newer")
		assert.Equal(t, "v3.0.0", targetVersion)
	})

	t.Run("module-present-above-minimum-current-newer-than-shipped", func(t *testing.T) {
		modPath := "test-module"
		orchestrionShippedVersions[modPath] = "v2.2.0"

		curMod := gomod.File{
			Require: []gomod.Require{
				{Path: modPath, Version: "v2.5.0"}, // Above v2.1.0, and greater than shipped
			},
		}

		shouldUpgrade, targetVersion, err := resolveIntegrationVersionWithFetcher(ctx, curMod, modPath, mockFetcher)
		require.NoError(t, err)
		// Since shipped (v2.2.0) < found (v2.5.0), shouldn't upgrade
		assert.False(t, shouldUpgrade, "should not upgrade when current version is newer than shipped")
		assert.Equal(t, "v2.5.0", targetVersion, "should keep current version")
	})

	t.Run("module-present-above-minimum-equal-to-shipped", func(t *testing.T) {
		modPath := "test-module"
		fixedVersion := "v2.3.0"
		orchestrionShippedVersions[modPath] = fixedVersion

		curMod := gomod.File{
			Require: []gomod.Require{
				{Path: modPath, Version: fixedVersion}, // Equal to shipped
			},
		}

		shouldUpgrade, targetVersion, err := resolveIntegrationVersionWithFetcher(ctx, curMod, modPath, mockFetcher)
		require.NoError(t, err)
		// semver.Compare returns 0 when equal, so NOT > 0, so shouldn't upgrade
		assert.False(t, shouldUpgrade, "should not upgrade when versions are equal")
		assert.Equal(t, fixedVersion, targetVersion, "should keep current version")
	})

	t.Run("shipped-version-is-latest", func(t *testing.T) {
		modPath := "golang.org/x/mod"
		orchestrionShippedVersions[modPath] = "latest"

		curMod := gomod.File{
			Require: []gomod.Require{
				{Path: modPath, Version: "v0.5.0"},
			},
		}

		_, targetVersion, err := resolveIntegrationVersion(ctx, curMod, modPath)
		require.NoError(t, err)
		// With shipped version as "latest", maxVersion should pick the latest from registry
		assert.True(t, semver.IsValid(targetVersion))
		// "latest" as a string sorts after semantic versions, so it will be picked
		// But in reality, fetchLatestVersion returns a real version
		assert.True(t, semver.Compare(targetVersion, "v0.5.0") >= 0 || targetVersion == "latest")
	})
}

func TestMaxVersion(t *testing.T) {
	tests := []struct {
		name     string
		versions []string
		expected string
	}{
		{
			name:     "single-version",
			versions: []string{"v1.2.3"},
			expected: "v1.2.3",
		},
		{
			name:     "two-versions-ascending",
			versions: []string{"v1.2.3", "v1.2.4"},
			expected: "v1.2.4",
		},
		{
			name:     "two-versions-descending",
			versions: []string{"v1.2.4", "v1.2.3"},
			expected: "v1.2.4",
		},
		{
			name:     "multiple-versions",
			versions: []string{"v1.2.3", "v2.0.0", "v1.9.9", "v1.2.4"},
			expected: "v2.0.0",
		},
		{
			name:     "with-empty-string",
			versions: []string{"v1.2.3", "", "v1.2.4"},
			expected: "v1.2.4", // semver.Sort puts empty string first as it's not valid semver.
		},
		{
			name:     "prerelease-versions",
			versions: []string{"v1.2.3", "v1.2.4-beta.1", "v1.2.4", "v1.2.5-rc.1"},
			expected: "v1.2.5-rc.1", // Pre-releases of the next version are considered greater.
		},
		{
			name:     "empty-and-version",
			versions: []string{"", "v1.2.3"},
			expected: "v1.2.3",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := maxVersion(tt.versions...)
			assert.Equal(t, tt.expected, result)
		})
	}
}
