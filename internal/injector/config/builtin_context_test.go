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
	"path/filepath"
	"runtime"
	"slices"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestContextRuntimeJoinPointTargetsExist is a CI canary for unexported Go
// runtime implementation details used by runtime/context/orchestrion.yml.
func TestContextRuntimeJoinPointTargetsExist(t *testing.T) {
	pkg, err := build.Import("runtime", "", 0)
	require.NoError(t, err, "failed to resolve runtime via go/build.Import")
	t.Logf("checked %s runtime sources in %q", runtime.Version(), pkg.Dir)

	found := map[string]bool{"goexit1": false, "newproc": false, "newproc1": false}
	newprocCallsNewproc1 := false
	fset := token.NewFileSet()
	for _, name := range slices.Concat(pkg.GoFiles, pkg.CgoFiles) {
		path := filepath.Join(pkg.Dir, name)
		file, err := parser.ParseFile(fset, path, nil, parser.SkipObjectResolution)
		require.NoError(t, err, "failed to parse %q", path)

		for _, decl := range file.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Recv != nil {
				continue
			}
			if _, tracked := found[fn.Name.Name]; tracked {
				found[fn.Name.Name] = true
			}
			if fn.Name.Name == "newproc" {
				ast.Inspect(fn.Body, func(node ast.Node) bool {
					call, ok := node.(*ast.CallExpr)
					if !ok {
						return true
					}
					if ident, ok := call.Fun.(*ast.Ident); ok && ident.Name == "newproc1" {
						newprocCallsNewproc1 = true
					}
					return true
				})
			}
		}
	}

	for name, ok := range found {
		require.True(t, ok, "runtime.%s not found in %s; update runtime/context join points", name, runtime.Version())
	}
	require.True(t, newprocCallsNewproc1,
		"runtime.newproc no longer calls newproc1 in %s; context must attach before the child is queued", runtime.Version())
}
