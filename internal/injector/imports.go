// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package injector

import (
	"go/token"
	"slices"
	"strconv"

	"github.com/DataDog/orchestrion/internal/injector/aspect"
	"github.com/DataDog/orchestrion/internal/injector/aspect/may"
	"github.com/DataDog/orchestrion/internal/log"
	"github.com/dave/dst"
	"github.com/dave/dst/dstutil"
)

// packageFilterAspects filters out aspects that imply imports not present in the import map and return a copy
// of the aspect array.
func (i *Injector) packageFilterAspects(aspects []*aspect.Aspect) []*aspect.Aspect {
	copyAspects := make([]*aspect.Aspect, len(aspects))
	copy(copyAspects, aspects)

	ctx := &may.PackageContext{
		ImportPath: i.ImportPath,
		ImportMap:  i.ImportMap,
		TestMain:   i.TestMain,
	}
	return slices.DeleteFunc(copyAspects, func(a *aspect.Aspect) bool {
		return a.JoinPoint.PackageMayMatch(ctx) == may.NeverMatch
	})
}

// canonicalizeImports works around the issue detailed in https://github.com/dave/dst/issues/45
// where dave/dst improperly handles multiple imports of the same package with different aliases,
// resulting in invalid output source code.
//
// To do so, it modifies the AST file so that it only includes a single import per path, using the
// first non-empty alias found.
func canonicalizeImports(file *dst.File) {
	specsByPath := importSpecsByImportPath(file.Imports)

	retain := filterExtraneousImports(specsByPath)

	file.Imports = file.Imports[:0] // Re-use the backing store, we'll keep <= what was there.
	for spec := range retain {
		file.Imports = append(file.Imports, spec)
	}

	filterDecls(file, retain)
}

func importSpecsByImportPath(imports []*dst.ImportSpec) map[string][]*dst.ImportSpec {
	byPath := make(map[string][]*dst.ImportSpec, len(imports))

	for _, imp := range imports {
		path, err := strconv.Unquote(imp.Path.Value)
		if err != nil {
			log.Debugf("failed to unquote import path %q: %v\n", imp.Path.Value, err)
			continue
		}
		byPath[path] = append(byPath[path], imp)
	}

	return byPath
}

func filterExtraneousImports(byPath map[string][]*dst.ImportSpec) map[*dst.ImportSpec]struct{} {
	result := make(map[*dst.ImportSpec]struct{}, len(byPath))

	for _, specs := range byPath {
		retain := specs[0]
		for _, spec := range specs[1:] {
			// Preference order is: spec.Name == nil < spec.Name.Name == "_" < spec.Name.Name != "_"
			if spec.Name == nil {
				continue
			}

			if retain.Name == nil && spec.Name != nil {
				retain = spec
				continue
			}

			if retain.Name.Name == "_" && spec.Name.Name != "_" {
				retain = spec
				continue
			}

			// We found a non-empty alias, no need to look further.
			break
		}
		result[retain] = struct{}{}
	}

	return result
}

func filterDecls(file *dst.File, retain map[*dst.ImportSpec]struct{}) {
	dstutil.Apply(
		file,
		func(csor *dstutil.Cursor) bool {
			switch node := csor.Node().(type) {
			case *dst.GenDecl:
				// Only visit the children of `import` declarations.
				return node.Tok == token.IMPORT
			case *dst.ImportSpec:
				// Filter out ImportSpec entries to keep only those in retain
				if _, ret := retain[node]; !ret {
					csor.Delete()
				}
				// No need to traverse children.
				return false
			case dst.Decl:
				// No need to visit any other kind of declaration
				return false
			default:
				// Visit other node types (e.g, the *ast.File)
				return true
			}
		},
		func(csor *dstutil.Cursor) bool {
			switch node := csor.Node().(type) {
			case *dst.GenDecl:
				if node.Tok != token.IMPORT {
					// Imports are before any other kind of declaration, we can abort traversal as soon as we
					// find a declaration that is not an `import` declaration.
					return false
				}

				if len(node.Specs) == 0 {
					csor.Delete()
				}
				// Proceed with the rest of the nodes (there may be more imports).
				return true
			case dst.Decl:
				// Imports are before any other kind of declaration, we can abort traversal as soon as we
				// find a declaration that is not an `import` declaration.
				return false
			default:
				// Proceed with the rest of the nodes (there may be imports down there).
				return true
			}
		},
	)
}
