// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package code

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestResolveInterfaceTypeByName tests the resolveInterfaceTypeByName function with various interface name formats.
func TestResolveInterfaceTypeByName(t *testing.T) {
	// Test cases for different interface types
	testCases := []struct {
		name           string
		interfaceName  string
		shouldSucceed  bool
		expectedMethod string
	}{
		{
			name:           "stdlib interface io.Reader",
			interfaceName:  "io.Reader",
			shouldSucceed:  true,
			expectedMethod: "Read",
		},
		{
			name:           "built-in error interface",
			interfaceName:  "error",
			shouldSucceed:  true,
			expectedMethod: "Error",
		},
		{
			name:          "invalid interface name",
			interfaceName: "123invalid",
			shouldSucceed: false,
		},
		{
			name:          "nonexistent interface",
			interfaceName: "nonexistent.Interface",
			shouldSucceed: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			iface, err := resolveInterfaceTypeByName(tc.interfaceName)

			if !tc.shouldSucceed {
				require.Error(t, err)
				require.Nil(t, iface)
				return
			}

			require.NoError(t, err)
			require.NotNil(t, iface)

			if tc.expectedMethod != "" {
				found := false
				for i := 0; i < iface.NumMethods(); i++ {
					if iface.Method(i).Name() == tc.expectedMethod {
						found = true
						break
					}
				}

				assert.True(t, found, "Interface does not contain expected method %s", tc.expectedMethod)
			}
		})
	}
}

// TestSplitPackageAndName tests the SplitPackageAndName function with various package name formats.
func TestSplitPackageAndName(t *testing.T) {
	testCases := []struct {
		name          string
		fullName      string
		expectedPkg   string
		expectedLocal string
	}{
		{
			name:          "standard library package",
			fullName:      "io.Reader",
			expectedPkg:   "io",
			expectedLocal: "Reader",
		},
		{
			name:          "built-in type",
			fullName:      "error",
			expectedPkg:   "",
			expectedLocal: "error",
		},
		{
			name:          "unqualified type",
			fullName:      "MyType",
			expectedPkg:   "",
			expectedLocal: "MyType",
		},
		{
			name:          "github import path",
			fullName:      "github.com/user/pkg.Type",
			expectedPkg:   "github.com/user/pkg",
			expectedLocal: "Type",
		},
		{
			name:          "versioned package",
			fullName:      "gopkg.in/pkg.v1.Type",
			expectedPkg:   "gopkg.in/pkg.v1",
			expectedLocal: "Type",
		},
		{
			name:          "complex domain with version and subpackage",
			fullName:      "gopkg.in/DataDog/dd-trace-go.v1/ddtrace.Span",
			expectedPkg:   "gopkg.in/DataDog/dd-trace-go.v1/ddtrace",
			expectedLocal: "Span",
		},
		{
			name:          "standard domain with version",
			fullName:      "github.com/DataDog/dd-trace-go/v2.Tracer",
			expectedPkg:   "github.com/DataDog/dd-trace-go/v2",
			expectedLocal: "Tracer",
		},
		{
			name:          "multiple dots in package name",
			fullName:      "k8s.io/client-go/kubernetes.Clientset",
			expectedPkg:   "k8s.io/client-go/kubernetes",
			expectedLocal: "Clientset",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			pkg, local := splitPackageAndName(tc.fullName)
			assert.Equal(t, tc.expectedPkg, pkg, "Package path should match")
			assert.Equal(t, tc.expectedLocal, local, "Local name should match")
		})
	}
}
