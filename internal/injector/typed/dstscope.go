// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package typed

import (
	"go/token"
	"sort"

	"github.com/dave/dst"
	"github.com/dave/dst/dstutil"
)

// dstScopes provides name-shadowing analysis over the final *dst.File, after all advice has been
// applied. The go/types scopes of the original package cannot answer these questions: they do
// not know about declarations that generated code introduces, and join-point scope checks run
// before later advice rewrites the file. The analysis below follows Go's scoping rules on the
// dst tree itself, using structural order (list positions) instead of source positions, since
// generated nodes carry no meaningful positions.
//
// Only names in the ordinary identifier namespace count. Labels, field names, and method names
// never shadow an import name.
type dstScopes struct {
	parents       map[dst.Node]dst.Node
	declaredNames map[string]bool // package-level names of this file, including injected ones
	indexes       map[dst.Node]*stmtIndex
}

func newDstScopes(file *dst.File) *dstScopes {
	parents := make(map[dst.Node]dst.Node)
	dstutil.Apply(file, func(csor *dstutil.Cursor) bool {
		if csor.Node() != nil && csor.Parent() != nil {
			parents[csor.Node()] = csor.Parent()
		}
		return true
	}, nil)

	s := &dstScopes{parents: parents, declaredNames: make(map[string]bool), indexes: make(map[dst.Node]*stmtIndex)}
	for _, decl := range file.Decls {
		switch decl := decl.(type) {
		case *dst.FuncDecl:
			// Method names occupy the method namespace of their type; only the function name
			// itself is a package-level identifier.
			s.declaredNames[decl.Name.Name] = true
		case *dst.GenDecl:
			for _, spec := range decl.Specs {
				switch spec := spec.(type) {
				case *dst.ValueSpec:
					for _, name := range spec.Names {
						s.declaredNames[name.Name] = true
					}
				case *dst.TypeSpec:
					s.declaredNames[spec.Name.Name] = true
				}
			}
		}
	}
	return s
}

// shadowedAt reports whether name is declared in any scope that encloses ref, so that name does
// not resolve to a file-scope import at ref's position. ref must be a node of the file this
// analysis was built for.
func (s *dstScopes) shadowedAt(ref dst.Node, name string) bool {
	if name == "" || name == "_" {
		return false
	}

	child := ref
	for {
		ancestor, ok := s.parents[child]
		if !ok || ancestor == nil {
			break
		}
		switch ancestor := ancestor.(type) {
		case *dst.File:
			return s.declaredNames[name]
		case *dst.FuncDecl:
			// Receiver, parameter, and result names scope the body only (Go scopes them to
			// the function body); type parameters scope the whole declaration.
			if child == ancestor.Body && (fieldListDeclares(ancestor.Recv, name) || funcValueDeclares(ancestor.Type, name)) {
				return true
			}
			if receiverTypeParamDeclares(ancestor.Recv, name) || typeParamsDeclare(ancestor.Type, name) {
				return true
			}
		case *dst.FuncLit:
			// A function literal has no type parameters; only its parameter and result
			// names scope its body.
			if child == ancestor.Body && funcValueDeclares(ancestor.Type, name) {
				return true
			}
		case *dst.TypeSpec:
			// The type parameters of a generic type declaration scope the whole declaration,
			// including the underlying type expression.
			if fieldListDeclares(ancestor.TypeParams, name) {
				return true
			}
		case *dst.BlockStmt:
			if s.listShadows(ancestor.List, ancestor, child, name) {
				return true
			}
		case *dst.IfStmt:
			// The init statement scopes the condition and both branches, but not its own
			// right-hand side.
			if child != ancestor.Init && stmtDeclares(ancestor.Init, name) {
				return true
			}
		case *dst.ForStmt:
			if child != ancestor.Init && stmtDeclares(ancestor.Init, name) {
				return true
			}
		case *dst.RangeStmt:
			// The range variables scope the loop body, but not the range expression.
			if child != ancestor.X && (identIs(ancestor.Key, name) || identIs(ancestor.Value, name)) {
				return true
			}
		case *dst.SwitchStmt:
			if child != ancestor.Init && stmtDeclares(ancestor.Init, name) {
				return true
			}
		case *dst.TypeSwitchStmt:
			// The init statement scopes the whole statement, but not its own right-hand side.
			// The guard variable scopes each clause body only; the walk assesses it when it
			// passes through the clause below.
			if child != ancestor.Init && stmtDeclares(ancestor.Init, name) {
				return true
			}
		case *dst.CaseClause:
			// Case expressions precede the body declarations, so a body declaration never
			// shadows a reference in the case list.
			if s.listShadows(ancestor.Body, ancestor, child, name) {
				return true
			}
			// The type-switch guard variable scopes each clause body, but the implicit clause
			// block opens only after the case list, so a reference in a case type is not
			// covered by it.
			if _, inBody := s.index(ancestor.Body, ancestor).positions[child]; inBody {
				if guard := s.typeSwitchGuard(ancestor); guard != nil && stmtDeclares(guard, name) {
					return true
				}
			}
		case *dst.CommClause:
			// The comm statement (e.g. `case str := <-ch:`) scopes the names it declares to
			// the clause body, but not to its own right-hand side.
			if child != ancestor.Comm && stmtDeclares(ancestor.Comm, name) {
				return true
			}
			if s.listShadows(ancestor.Body, ancestor, child, name) {
				return true
			}
		}
		child = ancestor
	}

	// The reference is not nested in the file (e.g. it is the file itself): no lexical scope
	// applies.
	return false
}

// stmtIndex indexes one statement list for shadow queries: the position of every element, and
// per name, the sorted positions of the elements that declare it. Building it costs one pass
// over the list; every query is then a binary search, so a whole file costs linear work plus a
// logarithmic factor per reference instead of a nested walk.
type stmtIndex struct {
	positions map[dst.Node]int
	declared  map[string][]int
}

// index returns the shadow index of a statement list, building it on first use. The owner
// node keys the cache.
func (s *dstScopes) index(list []dst.Stmt, owner dst.Node) *stmtIndex {
	if idx := s.indexes[owner]; idx != nil {
		return idx
	}
	idx := &stmtIndex{
		positions: make(map[dst.Node]int, len(list)),
		declared:  make(map[string][]int),
	}
	for i, stmt := range list {
		if stmt == nil {
			continue
		}
		idx.positions[stmt] = i
		for _, declared := range stmtDeclaredNames(stmt) {
			idx.declared[declared] = append(idx.declared[declared], i)
		}
	}
	s.indexes[owner] = idx
	return idx
}

// typeSwitchGuard returns the guard statement of the type switch this clause belongs to, or
// nil when the clause is not a type-switch case.
func (s *dstScopes) typeSwitchGuard(clause *dst.CaseClause) dst.Stmt {
	body, ok := s.parents[clause].(*dst.BlockStmt)
	if !ok {
		return nil
	}
	ts, ok := s.parents[body].(*dst.TypeSwitchStmt)
	if !ok {
		return nil
	}
	return ts.Assign
}

// listShadows reports whether name is declared by an element of list before the element that
// contains child. A statement's declared names cover everything after it in the same list, and
// nothing before it; the containing element's own declarations do not cover references inside
// its right-hand side, which inner scopes have already assessed on the way up. If child is not
// an element of the list, nothing in the list precedes it.
func (s *dstScopes) listShadows(list []dst.Stmt, owner dst.Node, child dst.Node, name string) bool {
	idx := s.index(list, owner)

	i, contained := idx.positions[child]
	if !contained {
		return false
	}
	// The declaring elements are sorted; name is shadowed when any of them comes before the
	// containing element.
	return sort.SearchInts(idx.declared[name], i) > 0
}

// stmtDeclares reports whether a statement declares name at the block level that contains it.
// Constructs that scope their own declarations (if/for/range/switch/select and nested blocks)
// declare nothing at this level.
func stmtDeclares(stmt dst.Stmt, name string) bool {
	if stmt == nil {
		return false
	}
	for _, declared := range stmtDeclaredNames(stmt) {
		if declared == name {
			return true
		}
	}
	return false
}

// stmtDeclaredNames returns the block-level names a statement declares.
func stmtDeclaredNames(stmt dst.Stmt) []string {
	switch stmt := stmt.(type) {
	case *dst.AssignStmt:
		if stmt.Tok != token.DEFINE {
			return nil
		}
		var names []string
		for _, lhs := range stmt.Lhs {
			if ident, ok := lhs.(*dst.Ident); ok && ident.Name != "_" && ident.Name != "" {
				names = append(names, ident.Name)
			}
		}
		return names
	case *dst.DeclStmt:
		if genDecl, ok := stmt.Decl.(*dst.GenDecl); ok {
			var names []string
			for _, spec := range genDecl.Specs {
				switch spec := spec.(type) {
				case *dst.ValueSpec:
					for _, ident := range spec.Names {
						if ident.Name != "_" && ident.Name != "" {
							names = append(names, ident.Name)
						}
					}
				case *dst.TypeSpec:
					names = append(names, spec.Name.Name)
				}
			}
			return names
		}
	case *dst.LabeledStmt:
		// A label occupies a separate namespace; the labeled statement's own declarations
		// still count at this level.
		return stmtDeclaredNames(stmt.Stmt)
	}
	return nil
}

// receiverTypeParamDeclares reports whether the receiver declares a type parameter with the
// given name. Go declares receiver type parameters inside the receiver type expression: they are
// the identifier arguments of its index list (go/types collects them with the same syntactic
// rule). Arguments that are not plain identifiers are instantiations, not declarations.
func receiverTypeParamDeclares(fl *dst.FieldList, name string) bool {
	if fl == nil {
		return false
	}
	for _, field := range fl.List {
		receiver := field.Type
		if star, ok := receiver.(*dst.StarExpr); ok {
			receiver = star.X
		}
		switch receiver := receiver.(type) {
		case *dst.IndexExpr:
			if identIs(receiver.Index, name) {
				return true
			}
		case *dst.IndexListExpr:
			for _, index := range receiver.Indices {
				if identIs(index, name) {
					return true
				}
			}
		}
	}
	return false
}

// funcValueDeclares reports whether any parameter or result name of the function type is name.
// These names scope the body only, never the signature.
func funcValueDeclares(ft *dst.FuncType, name string) bool {
	if ft == nil {
		return false
	}
	return fieldListDeclares(ft.Params, name) || fieldListDeclares(ft.Results, name)
}

// typeParamsDeclare reports whether any type parameter name of the function type is name. Type
// parameters scope the whole declaration, signature included.
func typeParamsDeclare(ft *dst.FuncType, name string) bool {
	if ft == nil {
		return false
	}
	return fieldListDeclares(ft.TypeParams, name)
}

func fieldListDeclares(fl *dst.FieldList, name string) bool {
	if fl == nil {
		return false
	}
	for _, field := range fl.List {
		for _, ident := range field.Names {
			if ident.Name == name {
				return true
			}
		}
	}
	return false
}

func identIs(expr dst.Expr, name string) bool {
	ident, ok := expr.(*dst.Ident)
	return ok && ident.Name == name
}
