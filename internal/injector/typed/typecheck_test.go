// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package typed

import (
	"go/importer"
	"go/types"
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

// TestTypeImplements tests the typeImplements function, specifically its method-name
// fallback for the cross-importer package identity mismatch that affects context.Context.
func TestTypeImplements(t *testing.T) {
	t.Run("cross-package identity failure proves fallback is needed", func(t *testing.T) {
		// Create two packages with the same import path but different pointers.
		// This simulates what happens when importer.Default() and importer.ForCompiler
		// both import the same package: they produce different *types.Package objects.
		pkg1 := types.NewPackage("mypkg", "mypkg")
		pkg2 := types.NewPackage("mypkg", "mypkg")

		// Create a named type in each package with the same name (e.g., time.Time).
		namedInPkg1 := types.NewNamed(types.NewTypeName(0, pkg1, "Token", nil), types.NewStruct(nil, nil), nil)
		namedInPkg2 := types.NewNamed(types.NewTypeName(0, pkg2, "Token", nil), types.NewStruct(nil, nil), nil)

		// Build an interface with a method that returns *Token from pkg1.
		ifacePkg := types.NewPackage("ifacepkg", "ifacepkg")
		ifaceSig := types.NewSignatureType(
			nil, nil, nil,
			types.NewTuple(),
			types.NewTuple(types.NewVar(0, ifacePkg, "", types.NewPointer(namedInPkg1))),
			false,
		)
		ifaceMethod := types.NewFunc(0, ifacePkg, "Foo", ifaceSig)
		iface := types.NewInterfaceType([]*types.Func{ifaceMethod}, nil).Complete()

		// Build a concrete struct with method Foo() returning *Token from pkg2.
		implPkg := types.NewPackage("implpkg", "implpkg")
		implNamed := types.NewNamed(types.NewTypeName(0, implPkg, "Impl", nil), types.NewStruct(nil, nil), nil)
		implRecv := types.NewVar(0, implPkg, "s", types.NewPointer(implNamed))
		implSig := types.NewSignatureType(
			implRecv, nil, nil,
			types.NewTuple(),
			types.NewTuple(types.NewVar(0, implPkg, "", types.NewPointer(namedInPkg2))),
			false,
		)
		implNamed.AddMethod(types.NewFunc(0, implPkg, "Foo", implSig))
		implPtrType := types.NewPointer(implNamed)

		// types.Implements should fail: Token in pkg1 and pkg2 are different *types.TypeName
		// objects, so types.Identical returns false and types.Implements returns false.
		assert.False(t, types.Implements(implPtrType, iface),
			"types.Implements should return false due to cross-package type identity mismatch")

		// typeImplements should succeed: it finds method "Foo" by name via LookupFieldOrMethod.
		assert.True(t, typeImplements(implPtrType, iface),
			"typeImplements should return true using method-name fallback")
	})

	t.Run("real context.Context case with cross-importer time.Time", func(t *testing.T) {
		// Load context.Context via ResolveInterfaceTypeByName (uses importer.Default internally).
		contextIface, err := ResolveInterfaceTypeByName("context.Context")
		require.NoError(t, err)
		require.NotNil(t, contextIface)

		// Import time via a SECOND independent importer.Default().
		// This produces a different *types.Package for "time" than the one used inside
		// ResolveInterfaceTypeByName, so types.Identical fails for the two time.Time types.
		imp2 := importer.Default()
		timePkg, err := imp2.Import("time")
		require.NoError(t, err)
		timeType := timePkg.Scope().Lookup("Time").Type()

		// Build a concrete type implementing context.Context methods, but with
		// Deadline() returning time.Time from imp2 — incompatible with imp1's time.Time.
		pkg := types.NewPackage("test", "test")
		customCtxNamed := types.NewNamed(
			types.NewTypeName(0, pkg, "CustomContext", nil),
			types.NewStruct(nil, nil), nil,
		)
		ptrToCustomCtx := types.NewPointer(customCtxNamed)

		recv := func() *types.Var { return types.NewVar(0, pkg, "c", ptrToCustomCtx) }

		// Deadline() (deadline time.Time, ok bool) — uses imp2's time.Time
		customCtxNamed.AddMethod(types.NewFunc(0, pkg, "Deadline",
			types.NewSignatureType(recv(), nil, nil,
				types.NewTuple(),
				types.NewTuple(
					types.NewVar(0, pkg, "deadline", timeType),
					types.NewVar(0, pkg, "ok", types.Typ[types.Bool]),
				),
				false),
		))

		// Done() <-chan struct{}
		customCtxNamed.AddMethod(types.NewFunc(0, pkg, "Done",
			types.NewSignatureType(recv(), nil, nil,
				types.NewTuple(),
				types.NewTuple(types.NewVar(0, pkg, "",
					types.NewChan(types.RecvOnly, types.NewStruct(nil, nil)))),
				false),
		))

		// Err() error
		customCtxNamed.AddMethod(types.NewFunc(0, pkg, "Err",
			types.NewSignatureType(recv(), nil, nil,
				types.NewTuple(),
				types.NewTuple(types.NewVar(0, pkg, "", types.Universe.Lookup("error").Type())),
				false),
		))

		// Value(key any) any
		anyType := types.Universe.Lookup("any").Type()
		customCtxNamed.AddMethod(types.NewFunc(0, pkg, "Value",
			types.NewSignatureType(recv(), nil, nil,
				types.NewTuple(types.NewVar(0, pkg, "key", anyType)),
				types.NewTuple(types.NewVar(0, pkg, "", anyType)),
				false),
		))

		// types.Implements should fail because time.Time from imp2 is a different
		// *types.TypeName than time.Time from the importer used inside ResolveInterfaceTypeByName.
		assert.False(t, types.Implements(ptrToCustomCtx, contextIface),
			"types.Implements should return false due to cross-importer time.Time mismatch")

		// typeImplements should succeed: all 4 methods (Deadline, Done, Err, Value) exist by name.
		assert.True(t, typeImplements(ptrToCustomCtx, contextIface),
			"typeImplements should return true using method-name fallback for context.Context")
	})
}
