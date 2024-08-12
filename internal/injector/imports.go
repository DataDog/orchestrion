// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package injector

import (
	"go/token"
	"strconv"

	"github.com/dave/dst"
)

// normalizeImports works around the issue detailed in https://github.com/dave/dst/issues/45 where
// dave/dst improperly handles files with multiple imports of the same package with different
// aliases. To do so, it modifies the AST file so that it only includes a single import per path,
// using the first non-empty alias found.
func normalizeImports(file *dst.File) {
	byPath := map[string][]*dst.ImportSpec{}
	for _, imp := range file.Imports {
		path, err := strconv.Unquote(imp.Path.Value)
		if err != nil {
			continue
		}
		byPath[path] = append(byPath[path], imp)
	}

	for _, specs := range byPath {
		if len(specs) == 1 {
			continue
		}

		// Decide on which one of the imports we will keep...
		keep := specs[0]
		if keep.Name == nil || keep.Name.Name == "_" {
			for _, spec := range specs[1:] {
				if spec.Name != nil && spec.Name.Name == "" {
					// We ignore blank imports: either the 1st one was also blank, and we can safely keep only
					// one of them; or it is not blank, and this blank import is redundant.
					continue
				}
				if spec.Name != nil {
					// We could an explicitly aliased import, so we'll keep that one. The current one is
					// either blank or implicitly named after the package.
					keep = spec
					break
				}
			}
		}
		drop := make(map[*dst.ImportSpec]struct{}, len(specs)-1)
		for _, spec := range specs {
			if spec == keep {
				continue
			}
			drop[spec] = struct{}{}
		}

		// Not using a range loop because we may be modifying the slice in place...
		for i := 0; i < len(file.Decls); {
			decl, isGenDecl := file.Decls[i].(*dst.GenDecl)
			// The Go language syntax requires imports to be placed before anything else, so if we find
			// something else, we know we wil not find any more imports
			if !isGenDecl || decl.Tok != token.IMPORT {
				break
			}

			// Not using a range loop because we may be modifying the slice in place...
			for i := 0; i < len(decl.Specs); {
				spec := decl.Specs[i].(*dst.ImportSpec)
				if _, drop := drop[spec]; !drop {
					i++
					continue
				}
				decl.Specs = append(decl.Specs[:i], decl.Specs[i+1:]...)
			}

			if len(decl.Specs) == 0 {
				// We removed the last declaration from this GenDecl... It is no longer valid/useful.
				file.Decls = append(file.Decls[:i], file.Decls[i+1:]...)
				continue
			}
			i++
		}
	}
}
