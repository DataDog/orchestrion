// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package injector

import (
	"go/token"
	"strconv"

	_ "unsafe"

	"github.com/dave/dst"
	"github.com/dave/dst/dstutil"
)

// canonicalizeImports works around the issue detailed in https://github.com/dave/dst/issues/45
// where dave/dst improperly handles multiple imports of the same package with different aliases,
// resulting in invalid output source code.
//
// To do so, it modifies the AST file so that it only includes a single import per path, using the
// first non-empty alias found.
func canonicalizeImports(file *dst.File) {
	specsByPath := importSpecsByImportPath(file)

	retain := filterExtraneousImports(specsByPath)

	file.Imports = make([]*dst.ImportSpec, 0, len(retain))
	for spec := range retain {
		file.Imports = append(file.Imports, spec)
	}

	filterDecls(file, retain)
}

func importSpecsByImportPath(file *dst.File) map[string][]*dst.ImportSpec {
	byPath := make(map[string][]*dst.ImportSpec, len(file.Imports))

	for _, imp := range file.Imports {
		path, err := strconv.Unquote(imp.Path.Value)
		if err != nil {
			continue
		}
		list := append(byPath[path], imp)
		byPath[path] = list
	}

	return byPath
}

func filterExtraneousImports(byPath map[string][]*dst.ImportSpec) map[*dst.ImportSpec]struct{} {
	result := make(map[*dst.ImportSpec]struct{}, len(byPath))

	for _, specs := range byPath {
		retain := specs[0]
		for _, spec := range specs[1:] {
			if spec.Name == nil || spec.Name.Name == "_" {
				continue
			}
			retain = spec
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
				return node.Tok == token.IMPORT
			case *dst.ImportSpec:
				if _, ret := retain[node]; ret {
					return false
				}
				csor.Delete()
			}
			return false
		},
		func(csor *dstutil.Cursor) bool {
			decl, ok := csor.Node().(*dst.GenDecl)
			if !ok || decl.Tok != token.IMPORT {
				return false
			}

			if len(decl.Specs) == 0 {
				csor.Delete()
				return false
			}

			return true
		},
	)
}
