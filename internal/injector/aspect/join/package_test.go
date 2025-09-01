// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package join

import (
	gocontext "context"
	"go/types"
	"testing"

	"github.com/dave/dst"
	"github.com/goccy/go-yaml"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/DataDog/orchestrion/internal/injector/aspect/context"
	"github.com/DataDog/orchestrion/internal/injector/aspect/may"
)

func TestPackageFilterGlobMatch(t *testing.T) {
	tests := []struct {
		name        string
		pattern     string
		importPath  string
		shouldMatch bool
	}{
		// Exact matches
		{
			name:        "exact match",
			pattern:     "github.com/myorg/mypackage",
			importPath:  "github.com/myorg/mypackage",
			shouldMatch: true,
		},
		{
			name:        "exact no match",
			pattern:     "github.com/myorg/mypackage",
			importPath:  "github.com/myorg/other",
			shouldMatch: false,
		},

		// Wildcard * tests (matches any sequence of non-separator characters)
		{
			name:        "wildcard at end",
			pattern:     "github.com/myorg/*",
			importPath:  "github.com/myorg/mypackage",
			shouldMatch: true,
		},
		{
			name:        "wildcard in middle",
			pattern:     "github.com/*/mypackage",
			importPath:  "github.com/myorg/mypackage",
			shouldMatch: true,
		},
		{
			name:        "multiple wildcards",
			pattern:     "github.com/*/*/*",
			importPath:  "github.com/myorg/service/api",
			shouldMatch: true,
		},
		{
			name:        "partial wildcard at end",
			pattern:     "github.com/myorg/my*",
			importPath:  "github.com/myorg/mypackage",
			shouldMatch: true,
		},

		// Question mark ? tests (matches any single non-separator character)
		{
			name:        "single question mark",
			pattern:     "github.com/myorg/service?",
			importPath:  "github.com/myorg/service1",
			shouldMatch: true,
		},
		{
			name:        "question mark no match",
			pattern:     "github.com/myorg/service?",
			importPath:  "github.com/myorg/service12",
			shouldMatch: false,
		},
		{
			name:        "question mark not match on separator",
			pattern:     "github.com/myorg?service1",
			importPath:  "github.com/myorg/service1",
			shouldMatch: false,
		},

		// Character class tests [class]
		{
			name:        "character class range",
			pattern:     "github.com/myorg/service[0-9]",
			importPath:  "github.com/myorg/service5",
			shouldMatch: true,
		},
		{
			name:        "character class no match",
			pattern:     "github.com/myorg/service[0-9]",
			importPath:  "github.com/myorg/servicea",
			shouldMatch: false,
		},

		// No cross-separator matching
		{
			name:        "wildcard cannot cross separators",
			pattern:     "github.com/myorg/*",
			importPath:  "github.com/myorg/service/api",
			shouldMatch: false,
		},

		// Globstar ** tests (recursive matching across path segments)
		{
			name:        "globstar matches deep paths",
			pattern:     "github.com/myorg/**/internal/*",
			importPath:  "github.com/myorg/service/internal/api",
			shouldMatch: true,
		},
		{
			name:        "globstar matches immediate child",
			pattern:     "github.com/myorg/**/internal",
			importPath:  "github.com/myorg/internal",
			shouldMatch: true,
		},
		{
			name:        "globstar matches multiple levels",
			pattern:     "github.com/**/internal/**",
			importPath:  "github.com/myorg/service/internal/api/v1",
			shouldMatch: true,
		},
		{
			name:        "globstar no match wrong prefix",
			pattern:     "github.com/myorg/**/internal/*",
			importPath:  "github.com/other/service/internal/api",
			shouldMatch: false,
		},
		{
			name:        "globstar trailing matches anything",
			pattern:     "github.com/myorg/**",
			importPath:  "github.com/myorg/service/api/v1/handler",
			shouldMatch: true,
		},
		{
			name:        "globstar leading matches anything",
			pattern:     "**/internal/*",
			importPath:  "github.com/myorg/service/internal/api",
			shouldMatch: true,
		},
		{
			name:        "just globstar matches everything",
			pattern:     "**",
			importPath:  "github.com/myorg/service/api/v1",
			shouldMatch: true,
		},

		// Edge cases
		{
			name:        "empty pattern",
			pattern:     "",
			importPath:  "github.com/myorg",
			shouldMatch: false,
		},
		{
			name:        "empty import path",
			pattern:     "github.com/myorg",
			importPath:  "",
			shouldMatch: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pf := PackageFilter(false, tt.pattern)
			result := pf.matchesPattern(tt.importPath)
			assert.Equal(t, tt.shouldMatch, result, "Pattern %q should match %q: %v", tt.pattern, tt.importPath, tt.shouldMatch)
		})
	}
}

func TestPackageFilterMatches(t *testing.T) {
	// Mock context for testing
	mockCtx := &mockAspectContext{
		importPath: "github.com/myorg/service",
	}

	tests := []struct {
		name        string
		pattern     string
		shouldMatch bool
	}{
		{
			name:        "exact match",
			pattern:     "github.com/myorg/service",
			shouldMatch: true,
		},
		{
			name:        "wildcard match",
			pattern:     "github.com/myorg/*",
			shouldMatch: true,
		},
		{
			name:        "complex wildcard match",
			pattern:     "github.com/*/service",
			shouldMatch: true,
		},
		{
			name:        "no match",
			pattern:     "github.com/other/*",
			shouldMatch: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pf := PackageFilter(false, tt.pattern)
			result := pf.Matches(mockCtx)
			assert.Equal(t, tt.shouldMatch, result)
		})
	}
}

func TestPackageFilterPackageMayMatch(t *testing.T) {
	tests := []struct {
		name       string
		pattern    string
		importPath string
		expected   may.MatchType
	}{
		{
			name:       "exact match",
			pattern:    "github.com/myorg/service",
			importPath: "github.com/myorg/service",
			expected:   may.Match,
		},
		{
			name:       "wildcard match",
			pattern:    "github.com/myorg/*",
			importPath: "github.com/myorg/service",
			expected:   may.Match,
		},
		{
			name:       "no match",
			pattern:    "github.com/other/*",
			importPath: "github.com/myorg/service",
			expected:   may.NeverMatch,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pf := PackageFilter(false, tt.pattern)
			ctx := &may.PackageContext{
				ImportPath: tt.importPath,
			}
			result := pf.PackageMayMatch(ctx)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestPackageFilterRootModule(t *testing.T) {
	tests := []struct {
		name        string
		root        bool
		pattern     string
		importPath  string
		shouldMatch bool
	}{
		{
			name:        "no root filter",
			root:        false,
			pattern:     "*",
			importPath:  "github.com/myorg/service",
			shouldMatch: true,
		},
		{
			name:        "root module with internal package",
			root:        true,
			pattern:     "internal/*",
			importPath:  "internal/service",
			shouldMatch: true, // This test will depend on actual module setup
		},
		{
			name:        "root module with external package",
			root:        true,
			pattern:     "*",
			importPath:  "github.com/external/service",
			shouldMatch: false, // External packages shouldn't match root filter
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pf := PackageFilter(tt.root, tt.pattern)
			result := pf.matchesPattern(tt.importPath)

			if tt.name == "root module with internal package" || tt.name == "root module with external package" {
				// Skip root module tests as they depend on actual module setup
				t.Skip("Root module tests require actual Go module context")
			}

			assert.Equal(t, tt.shouldMatch, result)
		})
	}
}

func TestPackageFilterSpecificModule(t *testing.T) {
	tests := []struct {
		name        string
		root        bool
		pattern     string
		importPath  string
		shouldMatch bool
	}{
		{
			name:        "specific module with internal pattern",
			root:        false,
			pattern:     "github.com/myorg/mymodule/internal/*",
			importPath:  "github.com/myorg/mymodule/internal/service",
			shouldMatch: true,
		},
		{
			name:        "specific module no match",
			root:        false,
			pattern:     "github.com/myorg/mymodule/internal/*",
			importPath:  "github.com/other/module/internal/service",
			shouldMatch: false,
		},
		{
			name:        "specific module exact match",
			root:        false,
			pattern:     "github.com/myorg/mymodule",
			importPath:  "github.com/myorg/mymodule",
			shouldMatch: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pf := PackageFilter(tt.root, tt.pattern)
			result := pf.matchesPattern(tt.importPath)
			assert.Equal(t, tt.shouldMatch, result, "Root %v with pattern %q should match %q: %v",
				tt.root, tt.pattern, tt.importPath, tt.shouldMatch)
		})
	}
}

func TestPackageFilterYAMLUnmarshaling(t *testing.T) {
	tests := []struct {
		name     string
		yaml     string
		expected packageFilter
		wantErr  bool
	}{
		{
			name:     "string syntax",
			yaml:     `package-filter: "github.com/myorg/**"`,
			expected: PackageFilter(false, "github.com/myorg/**"),
			wantErr:  false,
		},
		{
			name: "object syntax with root",
			yaml: `package-filter:
  root: true
  pattern: "internal/**"`,
			expected: PackageFilter(true, "internal/**"),
			wantErr:  false,
		},
		{
			name: "object syntax without root",
			yaml: `package-filter:
  pattern: "github.com/myorg/**"`,
			expected: PackageFilter(false, "github.com/myorg/**"),
			wantErr:  false,
		},
		{
			name: "object syntax missing pattern",
			yaml: `package-filter:
  root: true`,
			wantErr: true,
		},
		{
			name:    "invalid yaml",
			yaml:    `package-filter: [invalid]`,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var data map[string]any
			err := yaml.Unmarshal([]byte(tt.yaml), &data)
			require.NoError(t, err)

			node, err := yaml.ValueToNode(data["package-filter"])
			require.NoError(t, err)

			unmarshaler := unmarshalers["package-filter"]
			result, err := unmarshaler(gocontext.Background(), node)

			if tt.wantErr {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				pf, ok := result.(packageFilter)
				require.True(t, ok)
				assert.Equal(t, tt.expected, pf)
			}
		})
	}
}

func TestPackageFilterDebugCase(t *testing.T) {
	pattern := "github.com/ACME/*/*/my*"
	importPath := "github.com/ACME/internal/my-component/mypackage"

	pf := PackageFilter(false, pattern)
	result := pf.matchesPattern(importPath)

	t.Logf("Pattern: %q", pattern)
	t.Logf("Import Path: %q", importPath)
	t.Logf("Result: %v", result)

	// Test if this should match
	assert.True(t, result, "Pattern %q should match %q", pattern, importPath)
}

type mockAspectContext struct {
	importPath string
}

func (_ *mockAspectContext) Chain() *context.NodeChain       { return nil }
func (_ *mockAspectContext) Node() dst.Node                  { return nil }
func (_ *mockAspectContext) Parent() context.AspectContext   { return nil }
func (_ *mockAspectContext) Config(string) (string, bool)    { return "", false }
func (_ *mockAspectContext) File() *dst.File                 { return nil }
func (m *mockAspectContext) ImportPath() string              { return m.importPath }
func (_ *mockAspectContext) Package() string                 { return "" }
func (_ *mockAspectContext) TestMain() bool                  { return false }
func (_ *mockAspectContext) Release()                        {}
func (_ *mockAspectContext) ResolveType(dst.Expr) types.Type { return nil }
