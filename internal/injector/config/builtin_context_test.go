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

// TestContextGLSScrubJoinPointTargetExists is a CI canary for the
// "context.gls.scrub" aspect declared in builtin_context.go. That aspect's
// join point (join.ImportPath("runtime") + join.FunctionBody(join.Function(
// join.Name("goexit1")))) matches a real Go toolchain's runtime package by
// looking for a top-level, unexported function literally named `goexit1`.
// That function is not part of any API contract; if a future Go release
// renames, restructures, or removes it, the join point silently stops
// matching and the scrub statement is no longer woven in -- no build error,
// no failure from the existing (synthetic-runtime-backed) golden test.
//
// This test walks the real, currently running Go toolchain's actual GOROOT
// runtime package source (using the same build-constraint-aware file
// selection go/build.Import performs) looking for that exact declaration, so
// a break surfaces here instead of silently.
func TestContextGLSScrubJoinPointTargetExists(t *testing.T) {
	pkg, err := build.Import("runtime", "", 0)
	require.NoError(t, err, "failed to resolve the runtime package via go/build.Import")

	t.Logf("checked Go toolchain %s (GOROOT runtime package dir: %q)", runtime.Version(), pkg.Dir)

	fset := token.NewFileSet()
	found := false

	files := slices.Concat(pkg.GoFiles, pkg.CgoFiles)
	for _, name := range files {
		path := filepath.Join(pkg.Dir, name)

		file, err := parser.ParseFile(fset, path, nil, parser.SkipObjectResolution)
		require.NoError(t, err, "failed to parse %q", path)

		for _, decl := range file.Decls {
			funcDecl, ok := decl.(*ast.FuncDecl)
			if !ok || funcDecl.Recv != nil {
				// Not a plain (receiver-less) top-level function declaration.
				continue
			}
			if funcDecl.Name.Name == "goexit1" {
				found = true
				break
			}
		}
		if found {
			break
		}
	}

	require.True(t, found,
		"runtime.goexit1 not found in this Go toolchain's runtime package (checked GOROOT %q) — "+
			"the context.gls.scrub aspect's join point in internal/injector/config/builtin_context.go "+
			"depends on this function existing under that exact name; update the join point to match "+
			"wherever this logic now lives",
		pkg.Dir,
	)
}
