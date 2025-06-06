// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package typed

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
			iface, err := ResolveInterfaceTypeByName(tc.interfaceName)

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
		// Generic type test cases
		{
			name:          "simple generic type",
			fullName:      "iter.Seq[T]",
			expectedPkg:   "iter",
			expectedLocal: "Seq[T]",
		},
		{
			name:          "generic with qualified type parameter",
			fullName:      "iter.Seq[io.Reader]",
			expectedPkg:   "iter",
			expectedLocal: "Seq[io.Reader]",
		},
		{
			name:          "generic with multiple type parameters",
			fullName:      "maps.Map[string, int]",
			expectedPkg:   "maps",
			expectedLocal: "Map[string, int]",
		},
		{
			name:          "nested generic types",
			fullName:      "container.List[maps.Map[string, io.Reader]]",
			expectedPkg:   "container",
			expectedLocal: "List[maps.Map[string, io.Reader]]",
		},
		{
			name:          "generic with slice type parameter",
			fullName:      "sync.Pool[[]byte]",
			expectedPkg:   "sync",
			expectedLocal: "Pool[[]byte]",
		},
		{
			name:          "generic with pointer type parameter",
			fullName:      "atomic.Pointer[*sync.Mutex]",
			expectedPkg:   "atomic",
			expectedLocal: "Pointer[*sync.Mutex]",
		},
		{
			name:          "generic from versioned package",
			fullName:      "github.com/user/pkg/v2.Container[T]",
			expectedPkg:   "github.com/user/pkg/v2",
			expectedLocal: "Container[T]",
		},
		{
			name:          "complex generic with multiple qualified parameters",
			fullName:      "github.com/example/collections.Map[database/sql.DB, net/http.Client]",
			expectedPkg:   "github.com/example/collections",
			expectedLocal: "Map[database/sql.DB, net/http.Client]",
		},
		{
			name:          "generic with map type parameter",
			fullName:      "container.Set[map[string]interface{}]",
			expectedPkg:   "container",
			expectedLocal: "Set[map[string]interface{}]",
		},
		{
			name:          "generic with channel type parameter",
			fullName:      "async.Queue[chan error]",
			expectedPkg:   "async",
			expectedLocal: "Queue[chan error]",
		},
		{
			name:          "unqualified generic type",
			fullName:      "MyGeneric[T]",
			expectedPkg:   "",
			expectedLocal: "MyGeneric[T]",
		},
		{
			name:          "generic with function type parameter",
			fullName:      "functional.Option[func(io.Writer) error]",
			expectedPkg:   "functional",
			expectedLocal: "Option[func(io.Writer) error]",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			pkg, local := SplitPackageAndName(tc.fullName)
			assert.Equal(t, tc.expectedPkg, pkg, "Package path should match")
			assert.Equal(t, tc.expectedLocal, local, "Local name should match")
		})
	}
}
