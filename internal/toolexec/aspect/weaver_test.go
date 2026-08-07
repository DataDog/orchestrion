// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package aspect

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewWeaver(t *testing.T) {
	// The expected `$TOOLEXEC_IMPORTPATH` values are those produced by `(*load.Package).Desc` in
	// `cmd/go` for the packages Go builds when running `go test example.com/pkg`.
	for name, tc := range map[string]struct {
		toolexecImportPath string
		expected           Weaver
		isTestMain         bool
		packageUnderTest   string
	}{
		"ordinary": {
			toolexecImportPath: "example.com/pkg",
			expected:           Weaver{ImportPath: "example.com/pkg"},
		},
		"in-package test variant": {
			toolexecImportPath: "example.com/pkg [example.com/pkg.test]",
			expected:           Weaver{ImportPath: "example.com/pkg", Variant: "example.com/pkg.test"},
		},
		"external test package": {
			toolexecImportPath: "example.com/pkg_test [example.com/pkg.test]",
			expected:           Weaver{ImportPath: "example.com/pkg_test", Variant: "example.com/pkg.test"},
		},
		"dependency rebuilt for a test binary": {
			toolexecImportPath: "example.com/dep [example.com/pkg.test]",
			expected:           Weaver{ImportPath: "example.com/dep", Variant: "example.com/pkg.test"},
		},
		"test main": {
			toolexecImportPath: "example.com/pkg.test",
			expected:           Weaver{ImportPath: "example.com/pkg.test"},
			isTestMain:         true,
			packageUnderTest:   "example.com/pkg",
		},
		// Go also annotates packages it specializes for a main package's profile-guided optimization.
		"profile-guided optimization variant": {
			toolexecImportPath: "example.com/pkg [example.com/cmd/app]",
			expected:           Weaver{ImportPath: "example.com/pkg", Variant: "example.com/cmd/app"},
		},
		// A test variant of a package that is itself named `<something>.test` is not a test main.
		"test variant of a .test package": {
			toolexecImportPath: "example.com/pkg.test [example.com/pkg.test.test]",
			expected:           Weaver{ImportPath: "example.com/pkg.test", Variant: "example.com/pkg.test.test"},
		},
	} {
		t.Run(name, func(t *testing.T) {
			weaver := NewWeaver(tc.toolexecImportPath)
			require.Equal(t, tc.expected, weaver)
			assert.Equal(t, tc.isTestMain, weaver.isTestMain())
			if tc.isTestMain {
				assert.Equal(t, tc.packageUnderTest, weaver.packageUnderTest())
			}
		})
	}
}

func TestBehaviorOverrideAppliesToTestVariants(t *testing.T) {
	// Special cases are keyed on import paths, so they must be evaluated against the import path of the
	// package being built, and not the variant-annotated `$TOOLEXEC_IMPORTPATH` value; as failing to do
	// so would for example allow circular instrumentation of the tracer's own test variants.
	weaver := NewWeaver("github.com/DataDog/dd-trace-go/v2/ddtrace/tracer [github.com/DataDog/dd-trace-go/v2/ddtrace/tracer.test]")

	behavior, found := FindBehaviorOverride(weaver.ImportPath)
	require.True(t, found)
	assert.Equal(t, WeaveTracerInternal, behavior)
}
