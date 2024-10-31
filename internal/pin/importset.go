// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package pin

import (
	"errors"
	"fmt"
	"go/token"
	"slices"
	"strconv"

	"github.com/dave/dst"
	"github.com/dave/dst/dstutil"
)

type importSet struct {
	file     *dst.File
	imports  *dst.GenDecl
	imported map[string]*dst.ImportSpec
}

func importSetFrom(file *dst.File) *importSet {
	imported := make(map[string]*dst.ImportSpec, len(file.Imports))
	for _, spec := range file.Imports {
		if spec.Path == nil {
			// This should never happen as go/parser already verified this.
			panic(errors.New("encountered *dst.ImportSpec with nil path"))
		}
		path, err := strconv.Unquote(spec.Path.Value)
		if err != nil {
			// This should never happen as go/parser already verified this.
			panic(fmt.Errorf("encountered *dst.ImportSpec with invalid path %s: %w", spec.Path.Value, err))
		}
		imported[path] = spec
	}

	imports := firstImportIn(file)
	if imports == nil {
		imports = newImportDeclIn(file)
	}

	return &importSet{file: file, imports: imports, imported: imported}
}

func (s *importSet) Add(path string) (*dst.ImportSpec, bool) {
	if spec, found := s.imported[path]; found {
		return spec, false
	}

	newSpec := &dst.ImportSpec{
		Name: &dst.Ident{Name: "_"},
		Path: &dst.BasicLit{Kind: token.STRING, Value: strconv.Quote(path)},
	}

	s.file.Imports = append(s.file.Imports, newSpec)
	s.imports.Specs = append(s.imports.Specs, newSpec)
	s.imported[path] = newSpec

	return newSpec, true
}

func (s *importSet) Except(omit ...string) []string {
	list := make([]string, 0, len(s.imported))
	for path := range s.imported {
		if slices.Contains(omit, path) {
			continue
		}
		list = append(list, path)
	}
	return list
}

func (s *importSet) Find(path string) *dst.ImportSpec {
	return s.imported[path]
}

func (s *importSet) Remove(toRemove string) bool {
	removed := false

	// Remove actual import declarations from the AST.
	dstutil.Apply(
		s.file,
		func(csor *dstutil.Cursor) bool {
			switch node := csor.Node().(type) {
			case *dst.File, *dst.GenDecl:
				return true
			case *dst.ImportSpec:
				if node.Path == nil {
					return false
				}
				path, err := strconv.Unquote(node.Path.Value)
				if err != nil {
					return false
				}
				if path != toRemove {
					return false
				}
				csor.Delete()
				removed = true
				return false
			default:
				return false
			}
		},
		nil,
	)
	// Remove imports from the file-level registry.
	for i := 0; i < len(s.file.Imports); {
		spec := s.file.Imports[i]
		if spec.Path == nil {
			i++
			continue
		}
		path, err := strconv.Unquote(spec.Path.Value)
		if err != nil {
			i++
			continue
		}
		if path != toRemove {
			i++
			continue
		}
		s.file.Imports = append(s.file.Imports[:i], s.file.Imports[i+1:]...)
		removed = true
	}
	// Finally, remove from the quick-access set.
	delete(s.imported, toRemove)

	return removed
}

func firstImportIn(file *dst.File) *dst.GenDecl {
	if len(file.Decls) == 0 {
		return nil
	}

	genDecl, ok := file.Decls[0].(*dst.GenDecl)
	if !ok || genDecl.Tok != token.IMPORT {
		return nil
	}

	return genDecl
}

func newImportDeclIn(file *dst.File) *dst.GenDecl {
	const defaultSpecCap = 128

	decl := &dst.GenDecl{
		Decs: dst.GenDeclDecorations{
			NodeDecs: dst.NodeDecs{
				Before: dst.EmptyLine,
				Start: dst.Decorations{
					"// Imports in this file determine which tracer intergations are enabled in",
					"// orchestrion. New integrations can be automatically discovered by running",
					"// `orchestrion pin` again. You can also manually add new imports here to",
					"// enable additional integrations. When doing so, you can run `orchestrion pin`",
					"// to make sure manually added integrations are valid (i.e, the imported package",
					"// includes a valid `orchestrion.yml` file).",
				},
				After: dst.EmptyLine,
			},
		},
		Tok:    token.IMPORT,
		Lparen: true,
		Specs:  make([]dst.Spec, 0, defaultSpecCap),
		Rparen: true,
	}

	file.Decls = append(
		append(
			make([]dst.Decl, 0, len(file.Decls)+1),
			decl,
		),
		file.Decls...,
	)

	// If there's no imports array, pre-allocate one with the default capacity.
	if cap(file.Imports) == 0 {
		file.Imports = make([]*dst.ImportSpec, 0, defaultSpecCap)
	}

	return decl
}
