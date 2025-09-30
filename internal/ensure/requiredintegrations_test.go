// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package ensure

import (
	"context"
	"errors"
	"fmt"
	"os"
	"testing"

	"github.com/DataDog/orchestrion/internal/gomod"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/mod/semver"
)

// testContext creates a context with optional deadline from testing.T
func testContext(t *testing.T) context.Context {
	t.Helper()
	ctx := context.Background()
	if d, ok := t.Deadline(); ok {
		var cancel context.CancelFunc
		ctx, cancel = context.WithDeadline(ctx, d)
		t.Cleanup(cancel)
	}
	return ctx
}

// withShippedVersions temporarily sets shipped versions for testing and restores them after.
// This properly handles the atomic.Pointer and sync.Once initialization.
func withShippedVersions(t *testing.T, versions map[string]string) {
	t.Helper()

	// Force initialization if not already done
	_ = fetchShippedVersions()

	// Save original pointer
	originalPtr := orchestrionShippedVersions.Load()

	// Create new map with test versions merged with originals
	// This prevents breaking other tests that depend on real shipped versions
	testVersions := make(map[string]string)
	if originalPtr != nil {
		for k, v := range *originalPtr {
			testVersions[k] = v
		}
	}
	// Override with test-specific versions
	for k, v := range versions {
		testVersions[k] = v
	}

	// Set test versions
	orchestrionShippedVersions.Store(&testVersions)

	// Restore after test
	t.Cleanup(func() {
		orchestrionShippedVersions.Store(originalPtr)
	})
}

// mockVersionFetcher returns a versionFetcher that returns fixed versions
func mockVersionFetcher(version string) versionFetcher {
	return func(_ context.Context, _ string) (string, error) {
		return version, nil
	}
}

// mockVersionFetcherWithError returns a versionFetcher that always returns an error
func mockVersionFetcherWithError(err error) versionFetcher {
	return func(_ context.Context, modPath string) (string, error) {
		return "", fmt.Errorf("mock error for %s: %w", modPath, err)
	}
}

func TestFetchLatestVersion(t *testing.T) {
	ctx := testContext(t)

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

	t.Run("vendor-mode-compatibility", func(t *testing.T) {
		// Simulate CI environment with -mod=vendor in GOFLAGS
		originalGoFlags := os.Getenv("GOFLAGS")
		os.Setenv("GOFLAGS", "-mod=vendor")
		t.Cleanup(func() {
			if originalGoFlags == "" {
				os.Unsetenv("GOFLAGS")
			} else {
				os.Setenv("GOFLAGS", originalGoFlags)
			}
		})

		// This should still work even with -mod=vendor set
		version, err := fetchLatestVersion(ctx, "golang.org/x/mod")
		require.NoError(t, err, "fetchLatestVersion should work even with GOFLAGS=-mod=vendor")
		assert.NotEmpty(t, version)
		assert.True(t, semver.IsValid(version), "version %q should be valid semver", version)
	})
}

func TestResolveIntegrationVersion(t *testing.T) {
	ctx := testContext(t)

	tests := []struct {
		name              string
		shippedVersion    string
		currentVersion    string
		mockLatestVersion string
		found             bool
		wantUpgrade       bool
		wantVersion       string
		wantErr           bool
	}{
		{
			name:              "module-not-present",
			shippedVersion:    "v2.5.0",
			currentVersion:    "",
			mockLatestVersion: "v2.8.0",
			found:             false,
			wantUpgrade:       true,
			wantVersion:       "v2.8.0", // max(v2.5.0, v2.8.0)
		},
		{
			name:              "module-below-minimum-upgrade-to-latest",
			shippedVersion:    "v2.0.0",
			currentVersion:    "v1.5.0",
			mockLatestVersion: "v2.8.0",
			found:             true,
			wantUpgrade:       true,
			wantVersion:       "v2.8.0", // max(v2.0.0, v2.8.0)
		},
		{
			name:              "module-below-minimum-upgrade-to-shipped",
			shippedVersion:    "v2.5.0",
			currentVersion:    "v2.0.0",
			mockLatestVersion: "v2.3.0",
			found:             true,
			wantUpgrade:       true,
			wantVersion:       "v2.5.0", // max(v2.5.0, v2.3.0)
		},
		{
			name:              "module-above-minimum-shipped-newer",
			shippedVersion:    "v3.0.0",
			currentVersion:    "v2.5.0",
			mockLatestVersion: "v2.8.0",
			found:             true,
			wantUpgrade:       true,
			wantVersion:       "v3.0.0", // shipped > current, upgrade to shipped
		},
		{
			name:              "module-above-minimum-current-newer",
			shippedVersion:    "v2.2.0",
			currentVersion:    "v2.5.0",
			mockLatestVersion: "v2.8.0",
			found:             true,
			wantUpgrade:       false,
			wantVersion:       "v2.5.0", // shipped < current, keep current
		},
		{
			name:              "module-above-minimum-equal-to-shipped",
			shippedVersion:    "v2.5.0",
			currentVersion:    "v2.5.0",
			mockLatestVersion: "v2.8.0",
			found:             true,
			wantUpgrade:       false,
			wantVersion:       "v2.5.0", // shipped == current, keep current
		},
		{
			name:              "module-above-minimum-current-much-newer",
			shippedVersion:    "v2.2.0",
			currentVersion:    "v3.0.0",
			mockLatestVersion: "v2.8.0",
			found:             true,
			wantUpgrade:       false,
			wantVersion:       "v3.0.0", // current > shipped and latest, keep current
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			modPath := "test-module"

			withShippedVersions(t, map[string]string{
				modPath: tt.shippedVersion,
			})

			// Build current module state:
			var requires []gomod.Require
			if tt.found {
				requires = []gomod.Require{
					{Path: modPath, Version: tt.currentVersion},
				}
			}
			curMod := gomod.File{Require: requires}

			mockFetcher := mockVersionFetcher(tt.mockLatestVersion)

			shouldUpgrade, targetVersion, err := resolveIntegrationVersionWithFetcher(
				ctx, curMod, modPath, mockFetcher,
			)

			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.wantUpgrade, shouldUpgrade, "shouldUpgrade mismatch")
			assert.Equal(t, tt.wantVersion, targetVersion, "targetVersion mismatch")
		})
	}
}

func TestResolveIntegrationVersionWithRealNetwork(t *testing.T) {
	// These tests make real network calls and are kept separate for clarity
	ctx := testContext(t)

	t.Run("real-module-not-present", func(t *testing.T) {
		modPath := "golang.org/x/mod"
		withShippedVersions(t, map[string]string{
			modPath: "v0.15.0",
		})

		curMod := gomod.File{Require: nil}

		shouldUpgrade, targetVersion, err := resolveIntegrationVersion(ctx, curMod, modPath)
		require.NoError(t, err)
		assert.True(t, shouldUpgrade, "should upgrade when module is not present")
		assert.True(t, semver.IsValid(targetVersion))
		assert.GreaterOrEqual(t, semver.Compare(targetVersion, "v0.15.0"), 0)
	})

	t.Run("real-module-below-minimum", func(t *testing.T) {
		// This test validates error handling for non-existent modules
		modPath := "example.com/test-integration"
		withShippedVersions(t, map[string]string{
			modPath: "v2.5.0",
		})

		curMod := gomod.File{
			Require: []gomod.Require{
				{Path: modPath, Version: "v2.0.0"},
			},
		}

		_, _, err := resolveIntegrationVersion(ctx, curMod, modPath)
		// This should error because the module doesn't exist
		require.Error(t, err)
		assert.Contains(t, err.Error(), "fetching latest version")
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
			expected: "v1.2.4", // semver.Sort puts empty string first as it's not valid semver
		},
		{
			name:     "prerelease-versions",
			versions: []string{"v1.2.3", "v1.2.4-beta.1", "v1.2.4"},
			expected: "v1.2.4",
		},
		{
			name:     "prerelease-next-version",
			versions: []string{"v1.2.3", "v1.2.4-beta.1", "v1.2.4", "v1.2.5-rc.1"},
			expected: "v1.2.5-rc.1", // Pre-releases of the next version are considered greater
		},
		{
			name:     "empty-and-version",
			versions: []string{"", "v1.2.3"},
			expected: "v1.2.3",
		},
		{
			name:     "major-version-differences",
			versions: []string{"v1.99.99", "v2.0.0", "v0.1.0"},
			expected: "v2.0.0",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := maxVersion(tt.versions...)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestFetchShippedVersions(t *testing.T) {
	// Test that fetchShippedVersions is idempotent and properly initialized
	t.Run("idempotent-initialization", func(t *testing.T) {
		// First call
		versions1 := fetchShippedVersions()
		require.NotNil(t, versions1)

		// Second call should return same map content
		versions2 := fetchShippedVersions()
		require.NotNil(t, versions2)

		// Should return the same map content (maps are dereferenced from atomic.Pointer)
		assert.Equal(t, versions1, versions2)
	})

	t.Run("contains-expected-keys", func(t *testing.T) {
		versions := fetchShippedVersions()

		// Should contain dd-trace-go keys
		// Note: We can't assert exact versions as they change with each release
		for _, key := range []string{
			"github.com/DataDog/dd-trace-go/v2",
			"github.com/DataDog/dd-trace-go/orchestrion/all/v2",
		} {
			version, exists := versions[key]
			assert.True(t, exists, "expected key %q to exist", key)
			if exists {
				assert.True(t, semver.IsValid(version), "version %q for %q should be valid semver", version, key)
			}
		}
	})
}

func TestResolveIntegrationVersionEdgeCases(t *testing.T) {
	ctx := testContext(t)

	t.Run("fetcher-error", func(t *testing.T) {
		modPath := "test-module"
		withShippedVersions(t, map[string]string{
			modPath: "v2.5.0",
		})

		curMod := gomod.File{Require: nil}
		errorFetcher := mockVersionFetcherWithError(errors.New("network error"))

		_, _, err := resolveIntegrationVersionWithFetcher(ctx, curMod, modPath, errorFetcher)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "fetching latest version")
		assert.Contains(t, err.Error(), "network error")
	})

	t.Run("empty-shipped-version", func(t *testing.T) {
		modPath := "test-module"
		withShippedVersions(t, map[string]string{
			modPath: "", // Empty shipped version
		})

		curMod := gomod.File{Require: nil}
		mockFetcher := mockVersionFetcher("v2.5.0")

		shouldUpgrade, targetVersion, err := resolveIntegrationVersionWithFetcher(
			ctx, curMod, modPath, mockFetcher,
		)

		require.NoError(t, err)
		assert.True(t, shouldUpgrade)
		// maxVersion should handle empty string and pick v2.5.0
		assert.Equal(t, "v2.5.0", targetVersion)
	})
}
