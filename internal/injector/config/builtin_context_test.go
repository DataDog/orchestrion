// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package config

import (
	"go/ast"
	"go/build"
	"go/parser"
	"go/token"
	"go/types"
	"path/filepath"
	"runtime"
	"slices"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestContextRuntimeJoinPointTargetsExist is a CI canary for unexported Go
// runtime implementation details used by runtime/context/orchestrion.yml.
//
// It asserts that the set of functions that directly call runtime.newproc1 is
// exactly the known set below. Every goroutine in the runtime is created
// through one of these call sites: newproc and newcoro are woven by the
// aspects (go statements and iter.Pull coroutines, respectively), while
// debugCallWrap (debugger-injected calls) and synctestRun (testing/synctest
// bubble roots) are deliberately left unwoven and receive no context. A new Go
// release that adds another goroutine-creation path must fail here so that
// the propagation gap is a conscious decision rather than a silent escape.
func TestContextRuntimeJoinPointTargetsExist(t *testing.T) {
	pkg, err := build.Import("runtime", "", 0)
	require.NoError(t, err, "failed to resolve runtime via go/build.Import")
	t.Logf("checked %s runtime sources in %q", runtime.Version(), pkg.Dir)

	// Functions whose existence the aspects depend on (weave targets).
	found := map[string]bool{"goexit1": false, "newproc": false, "newproc1": false, "newcoro": false, "gdestroy": false}
	// The attach hook calls isSystemGoroutine(gp, false) to keep runtime-owned
	// goroutines from inheriting context; its signature is a compile-time
	// dependency of the injected template.
	isSystemGoroutineSignature := ""
	// The complete, exact set of direct newproc1 callers, classified by
	// whether propagation is woven into them. This must be updated (together
	// with the aspects and their documentation) whenever a Go release changes
	// goroutine creation.
	wovenCallers := map[string]bool{"newproc": false, "newcoro": false}
	unwovenCallers := map[string]bool{"debugCallWrap": false, "synctestRun": false}
	// The context.gls.scrub.destroy aspect references gdestroy's first
	// parameter by name.
	gdestroyParamName := ""
	fset := token.NewFileSet()
	for _, name := range slices.Concat(pkg.GoFiles, pkg.CgoFiles) {
		path := filepath.Join(pkg.Dir, name)
		file, err := parser.ParseFile(fset, path, nil, parser.SkipObjectResolution)
		require.NoError(t, err, "failed to parse %q", path)

		for _, decl := range file.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Recv != nil || fn.Body == nil {
				continue
			}
			if _, tracked := found[fn.Name.Name]; tracked {
				found[fn.Name.Name] = true
			}
			if fn.Name.Name == "gdestroy" {
				if params := fn.Type.Params; params != nil && len(params.List) > 0 && len(params.List[0].Names) > 0 {
					gdestroyParamName = params.List[0].Names[0].Name
				}
			}
			if fn.Name.Name == "isSystemGoroutine" {
				if fn.Type.Params == nil || len(fn.Type.Params.List) != 2 {
					t.Errorf("runtime.isSystemGoroutine no longer takes (*g, bool) in %s; the attach hook's no-system-goroutine guard references it by that shape", runtime.Version())
				} else {
					isSystemGoroutineSignature = types.ExprString(fn.Type.Params.List[0].Type) + "/" + types.ExprString(fn.Type.Params.List[1].Type)
				}
			}
			callsNewproc1 := false
			ast.Inspect(fn.Body, func(node ast.Node) bool {
				call, ok := node.(*ast.CallExpr)
				if !ok {
					return true
				}
				if ident, ok := call.Fun.(*ast.Ident); ok && ident.Name == "newproc1" {
					callsNewproc1 = true
				}
				return true
			})
			if callsNewproc1 {
				if _, woven := wovenCallers[fn.Name.Name]; woven {
					wovenCallers[fn.Name.Name] = true
				} else if _, unwoven := unwovenCallers[fn.Name.Name]; unwoven {
					unwovenCallers[fn.Name.Name] = true
				} else {
					t.Errorf("runtime.%s calls newproc1 but is neither woven nor a known-unwoven caller; "+
						"update runtime/context/orchestrion.yml (and this test) to classify it",
						fn.Name.Name)
				}
			}
		}
	}

	for name, ok := range found {
		require.True(t, ok, "runtime.%s not found in %s; update runtime/context join points", name, runtime.Version())
	}
	for name, ok := range wovenCallers {
		require.True(t, ok,
			"runtime.%s no longer calls newproc1 in %s; goroutine context propagation is broken for it",
			name, runtime.Version())
	}
	for name, ok := range unwovenCallers {
		require.True(t, ok,
			"runtime.%s no longer calls newproc1 in %s; if it was renamed or removed, update this test and the propagation scope docs",
			name, runtime.Version())
	}
	require.Equal(t, "gp", gdestroyParamName,
		"runtime.gdestroy's first parameter is no longer named 'gp' in %s; the context.gls.scrub.destroy aspect references it by name",
		runtime.Version())
	require.Equal(t, "*g/bool", isSystemGoroutineSignature,
		"runtime.isSystemGoroutine's parameter types are no longer (*g, bool) in %s; the attach hook calls it as such",
		runtime.Version())
}
