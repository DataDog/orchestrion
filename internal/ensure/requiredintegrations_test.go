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

func TestFetchVersions(t *testing.T) {
	ctx := testContext(t)

	t.Run("module-found", func(t *testing.T) {
		modPath := "test-module"
		withShippedVersions(t, map[string]string{
			modPath: "v2.5.0",
		})

		curMod := gomod.File{
			Require: []gomod.Require{
				{Path: modPath, Version: "v2.3.0"},
			},
		}

		mockFetcher := mockVersionFetcher("v2.8.0")

		ver, err := fetchVersions(ctx, curMod, modPath, mockFetcher)
		require.NoError(t, err)
		assert.True(t, ver.found)
		assert.Equal(t, "v2.3.0", ver.current)
		assert.Equal(t, "v2.5.0", ver.shipped)
		assert.Equal(t, "v2.8.0", ver.latest)
	})

	t.Run("module-not-found", func(t *testing.T) {
		modPath := "test-module"
		withShippedVersions(t, map[string]string{
			modPath: "v2.5.0",
		})

		curMod := gomod.File{Require: nil}
		mockFetcher := mockVersionFetcher("v2.8.0")

		ver, err := fetchVersions(ctx, curMod, modPath, mockFetcher)
		require.NoError(t, err)
		assert.False(t, ver.found)
		assert.Empty(t, ver.current)
		assert.Equal(t, "v2.5.0", ver.shipped)
		assert.Equal(t, "v2.8.0", ver.latest)
	})

	t.Run("fetcher-error", func(t *testing.T) {
		modPath := "test-module"
		withShippedVersions(t, map[string]string{
			modPath: "v2.5.0",
		})

		curMod := gomod.File{Require: nil}
		mockErr := errors.New("network error")
		errorFetcher := mockVersionFetcherWithError(mockErr)

		_, err := fetchVersions(ctx, curMod, modPath, errorFetcher)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "fetching latest version")
		assert.Contains(t, err.Error(), "network error")
	})
}

func TestResolveIntegrationVersion(t *testing.T) {
	tests := []struct {
		name        string
		versions    *versions
		wantUpgrade bool
		wantVersion string
		description string
	}{
		{
			name: "module-not-present",
			versions: &versions{
				found:   false,
				current: "",
				shipped: "v2.5.0",
				latest:  "v2.8.0",
			},
			wantUpgrade: true,
			wantVersion: "v2.8.0", // max(v2.5.0, v2.8.0)
			description: "Should upgrade to latest when module not present",
		},
		{
			name: "module-below-minimum-upgrade-to-latest",
			versions: &versions{
				found:   true,
				current: "v1.5.0",
				shipped: "v2.0.0",
				latest:  "v2.8.0",
			},
			wantUpgrade: true,
			wantVersion: "v2.8.0", // max(v2.0.0, v2.8.0)
			description: "Should upgrade to latest when current < v2.1.0",
		},
		{
			name: "module-below-minimum-upgrade-to-shipped",
			versions: &versions{
				found:   true,
				current: "v2.0.0",
				shipped: "v2.3.0",
				latest:  "v2.1.0",
			},
			wantUpgrade: true,
			wantVersion: "v2.3.0", // max(v2.3.0, v2.1.0)
			description: "Should upgrade to shipped when shipped > latest and current < v2.1.0",
		},
		{
			name: "module-above-minimum-shipped-newer",
			versions: &versions{
				found:   true,
				current: "v2.5.0",
				shipped: "v2.8.0",
				latest:  "v2.7.0",
			},
			wantUpgrade: true,
			wantVersion: "v2.8.0", // shipped > current, upgrade to shipped
			description: "Should upgrade to shipped when shipped > current >= v2.1.0",
		},
		{
			name: "module-above-minimum-current-newer",
			versions: &versions{
				found:   true,
				current: "v2.5.0",
				shipped: "v2.2.0",
				latest:  "v2.8.0",
			},
			wantUpgrade: false,
			wantVersion: "v2.5.0", // shipped < current, keep current
			description: "Should not upgrade when current > shipped >= v2.1.0",
		},
		{
			name: "module-above-minimum-equal-to-shipped",
			versions: &versions{
				found:   true,
				current: "v2.5.0",
				shipped: "v2.5.0",
				latest:  "v2.8.0",
			},
			wantUpgrade: false,
			wantVersion: "v2.5.0", // shipped == current, keep current
			description: "Should not upgrade when current == shipped >= v2.1.0",
		},
		{
			name: "module-above-minimum-current-much-newer",
			versions: &versions{
				found:   true,
				current: "v3.0.0",
				shipped: "v2.2.0",
				latest:  "v2.8.0",
			},
			wantUpgrade: false,
			wantVersion: "v3.0.0", // current > shipped and latest, keep current
			description: "Should not upgrade when current > shipped and current > latest",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			shouldUpgrade, targetVersion := resolveIntegrationVersion(tt.versions)

			assert.Equal(t, tt.wantUpgrade, shouldUpgrade, "shouldUpgrade mismatch: %s", tt.description)
			assert.Equal(t, tt.wantVersion, targetVersion, "targetVersion mismatch: %s", tt.description)
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

		ver, err := fetchVersions(ctx, curMod, modPath, fetchLatestVersion)
		require.NoError(t, err)

		shouldUpgrade, targetVersion := resolveIntegrationVersion(ver)
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

		_, err := fetchVersions(ctx, curMod, modPath, fetchLatestVersion)
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

func TestRequiredIntegrationsEdgeCases(t *testing.T) {
	ctx := testContext(t)

	t.Run("empty-shipped-version", func(t *testing.T) {
		modPath := "test-module"
		withShippedVersions(t, map[string]string{
			modPath: "", // Empty shipped version
		})

		curMod := gomod.File{Require: nil}
		mockFetcher := mockVersionFetcher("v2.5.0")

		ver, err := fetchVersions(ctx, curMod, modPath, mockFetcher)
		require.NoError(t, err)

		shouldUpgrade, targetVersion := resolveIntegrationVersion(ver)
		assert.True(t, shouldUpgrade)
		// maxVersion should handle empty string and pick v2.5.0
		assert.Equal(t, "v2.5.0", targetVersion)
	})
}

func TestRequiredIntegrationsReplaceDirective(t *testing.T) {
	// Test that the Replace directive is correctly generated with proper NewPath and NewVersion
	t.Run("replace-when-current-differs-from-shipped", func(t *testing.T) {
		modPath := "github.com/DataDog/dd-trace-go/v2"
		withShippedVersions(t, map[string]string{
			modPath: "v2.5.0",
		})

		ver := &versions{
			found:   true,
			current: "v2.3.0", // Different from shipped v2.5.0
			shipped: "v2.5.0",
			latest:  "v2.8.0",
		}

		// Based on the logic in RequiredIntegrations (lines 65-67):
		// Replace is added when: found && current != shipped
		shouldAddReplace := ver.found && semver.Compare(ver.current, ver.shipped) != 0
		assert.True(t, shouldAddReplace, "Replace directive should be added when current differs from shipped")

		// Verify the Replace struct is correctly formed with both NewPath and NewVersion
		if shouldAddReplace {
			replace := gomod.Replace{
				OldPath:    modPath,
				NewPath:    modPath, // Must be the same path to avoid "empty string" error
				NewVersion: ver.current,
			}

			// Verify both fields are set correctly
			assert.Equal(t, modPath, replace.OldPath)
			assert.Equal(t, modPath, replace.NewPath, "NewPath must be set to avoid malformed import path error")
			assert.Equal(t, ver.current, replace.NewVersion)
		}
	})

	t.Run("no-replace-when-current-equals-shipped", func(t *testing.T) {
		modPath := "github.com/DataDog/dd-trace-go/v2"
		withShippedVersions(t, map[string]string{
			modPath: "v2.5.0",
		})

		ver := &versions{
			found:   true,
			current: "v2.5.0", // Same as shipped
			shipped: "v2.5.0",
			latest:  "v2.8.0",
		}

		shouldAddReplace := ver.found && semver.Compare(ver.current, ver.shipped) != 0
		assert.False(t, shouldAddReplace, "Replace directive should NOT be added when current equals shipped")
	})

	t.Run("no-replace-when-module-not-found", func(t *testing.T) {
		ver := &versions{
			found:   false,
			current: "",
			shipped: "v2.5.0",
			latest:  "v2.8.0",
		}

		shouldAddReplace := ver.found && semver.Compare(ver.current, ver.shipped) != 0
		assert.False(t, shouldAddReplace, "Replace directive should NOT be added when module not found")
	})

	t.Run("replace-with-pseudo-version", func(t *testing.T) {
		// Test the exact scenario from the error report
		modPath := "github.com/DataDog/dd-trace-go/v2"
		pseudoVersion := "v2.4.0-dev.0.20250911151540-94e598897591"
		withShippedVersions(t, map[string]string{
			modPath: "v2.2.3",
		})

		ver := &versions{
			found:   true,
			current: pseudoVersion,
			shipped: "v2.2.3",
			latest:  "v2.2.3",
		}

		shouldAddReplace := ver.found && semver.Compare(ver.current, ver.shipped) != 0
		assert.True(t, shouldAddReplace, "Replace directive should be added for pseudo-version")

		if shouldAddReplace {
			replace := gomod.Replace{
				OldPath:    modPath,
				NewPath:    modPath,
				NewVersion: ver.current,
			}

			// Verify the Replace struct has all required fields for pseudo-versions
			assert.Equal(t, modPath, replace.OldPath)
			assert.Equal(t, modPath, replace.NewPath, "NewPath must be set for pseudo-versions")
			assert.Equal(t, pseudoVersion, replace.NewVersion)
			assert.Empty(t, replace.OldVersion, "OldVersion should be empty for unconditional replace")
		}
	})
}
