// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package context

import (
	"go/ast"
	"go/types"
	"sync"

	"github.com/dave/dst"
	"github.com/dave/dst/dstutil"

	"github.com/DataDog/orchestrion/internal/injector/typed"
)

type AspectContext interface {
	// Chain returns the node chain at this context.
	Chain() *NodeChain

	// Node returns the node represented by this context. Upon entering a join
	// point, this is the node being inspected. Upon entering advice, this is the
	// node being advised.
	Node() dst.Node

	// Parent returns an AspectContext representing the current node's parent.
	// Returns nil if the current node is the root of the AST (usually true of
	// the *dst.File node).
	Parent() AspectContext

	// Config loops up the node chain to find a value for the provided
	// configuration key.
	Config(string) (string, bool)

	// File provides direct access to the AST file containing the current node.
	File() *dst.File

	// ImportPath returns the import path for the package containing this node.
	ImportPath() string

	// Package returns the name of the package containing this node.
	Package() string

	// TestMain returns true if the current node is in a synthetic main package.
	TestMain() bool

	// Release returns this context to the memory pool so that it can be reused
	// later.
	Release()
}

type AdviceContext interface {
	AspectContext

	// Child creates a child of this context using the supplied node, property
	// name and index.
	Child(dst.Node, string, int) AdviceContext

	// ReplaceNode replaces the current AST node with the supplied one.
	ReplaceNode(dst.Node)

	// ParseSource parses Go source code from the provided bytes and returns a
	// *dst.File value.
	ParseSource([]byte) (*dst.File, error)

	// AddImport records a new synthetic import on this context.
	AddImport(path string, alias string) bool

	// AddLink records a new link-time requirement on this context.
	AddLink(string) bool

	// EnsureMinGoLang ensures that the current compile unit uses at least the
	// specified language level when passed to the compiler.
	EnsureMinGoLang(GoLangVersion)

	// ResolveType resolves a dst.Expr to its corresponding types.Type.
	ResolveType(dst.Expr) types.Type
}

type (
	context struct {
		*NodeChain
		cursor *dstutil.Cursor

		// Common to all contexts in the same hierarchy...
		file         *dst.File
		refMap       *typed.ReferenceMap
		minGoLang    *GoLangVersion
		sourceParser SourceParser
		importPath   string
		testMain     bool
		typeInfo     types.Info
		nodeMap      map[dst.Node]ast.Node
	}

	SourceParser interface {
		Parse(any) (*dst.File, error)
	}
)

var contextPool = sync.Pool{New: func() any { return new(context) }}

type ContextArgs struct {
	// Cursor denotes the current node and its context in the AST.
	Cursor *dstutil.Cursor
	// ImportPath is the fully qualified import path of the package containing the
	// current AST.
	ImportPath string
	// File is the AST of the file which the current node belongs in.
	File *dst.File
	// RefMap is the output reference map that will collect all synthetic
	// references added to the AST.
	RefMap *typed.ReferenceMap
	// SourceParser is used to parse generated source files.
	SourceParser SourceParser
	// MinGoLang is a pointer to the result value containing the minimum Go
	// language level required by the compile unit after it has been modified.
	MinGoLang *GoLangVersion
	// TestMain is true when injecting into a synthetic main package.
	TestMain bool
	// TypeInfo contains type information about the AST.
	TypeInfo types.Info
	// NodeMap maps dst.Node to ast.Node.
	NodeMap map[dst.Node]ast.Node
}

// Context returns a new [*context] instance that represents the ndoe at the
// provided cursor. The [context.Release] function should be called on values
// returned by this function to allow for memory re-use, which can significantly
// reduce allocations performed during AST traversal.
func (n *NodeChain) Context(args ContextArgs) *context {
	c, _ := contextPool.Get().(*context)
	*c = context{
		NodeChain:    n,
		cursor:       args.Cursor,
		file:         args.File,
		refMap:       args.RefMap,
		minGoLang:    args.MinGoLang,
		sourceParser: args.SourceParser,
		importPath:   args.ImportPath,
		testMain:     args.TestMain,
		typeInfo:     args.TypeInfo,
		nodeMap:      args.NodeMap,
	}

	return c
}

// Release returns the [*context] to the pool so that it can be reused later.
// Proper use can significantly reduce memory allocations perfomed during AST
// traversal.
func (c *context) Release() {
	*c = context{} // Zero it off
	contextPool.Put(c)
}

// Child returns a child context of this context, representing the provided node
// that is found at the specified property name or index. The [context.Release]
// function should be called on values returned by this function to allow for
// memory re-use, which can significantly reduce allocations performed during
// AST traversal.
func (c *context) Child(node dst.Node, property string, index int) AdviceContext {
	r, _ := contextPool.Get().(*context)
	*r = context{
		NodeChain: &NodeChain{
			parent: c.NodeChain,
			node:   node,
			name:   property,
			index:  index,
		},
		cursor:       nil,
		file:         c.file,
		refMap:       c.refMap,
		minGoLang:    c.minGoLang,
		sourceParser: c.sourceParser,
		importPath:   c.importPath,
		testMain:     c.testMain,
		typeInfo:     c.typeInfo,
		nodeMap:      c.nodeMap,
	}

	return r
}

// Chain returns the backing [*NodeChain] for this context. Using this to
// traverse the current node's ancestry is more efficient than using
// [context.Parent].
func (c *context) Chain() *NodeChain {
	return c.NodeChain
}

// Parent returns a new [*context] representing the parent of the current node.
// The [context.Release] function should be called on values returned by this
// function to allow for memory re-use, which can significantly reduce
// allocations performed during AST traversal.
func (c *context) Parent() AspectContext {
	parent := c.NodeChain.parent
	if parent == nil {
		return nil
	}

	p, _ := contextPool.Get().(*context)
	*p = context{
		NodeChain:  parent,
		file:       c.file,
		refMap:     c.refMap,
		importPath: c.importPath,
		typeInfo:   c.typeInfo,
		nodeMap:    c.nodeMap,
	}

	return p
}

func (c *context) ReplaceNode(newNode dst.Node) {
	if c.cursor == nil {
		panic("illegal attempt to replace a node without a cursor!")
	}
	c.cursor.Replace(newNode)
	c.node = newNode
}

func (c *context) File() *dst.File {
	return c.file
}

func (c *context) ImportPath() string {
	return c.importPath
}

func (c *context) Package() string {
	return c.file.Name.Name
}

func (c *context) TestMain() bool {
	return c.testMain
}

func (c *context) ParseSource(bytes []byte) (*dst.File, error) {
	return c.sourceParser.Parse(bytes)
}

func (c *context) AddImport(path string, name string) bool {
	nodeChain := []dst.Node{c.node}
	for p := c.NodeChain.parent; p != nil; p = p.parent {
		nodeChain = append(nodeChain, p.node)
	}

	return c.refMap.AddImport(c.file, nodeChain, path, name)
}

func (c *context) AddLink(path string) bool {
	return c.refMap.AddLink(c.file, path)
}

func (c *context) EnsureMinGoLang(lang GoLangVersion) {
	c.minGoLang.SetAtLeast(lang)
}

// ResolveType resolves a dst.Expr to its corresponding types.Type within the
// current context.
func (c *context) ResolveType(expr dst.Expr) types.Type {
	if expr == nil {
		return nil
	}

	// Convert dst.Expr to ast.Expr using the nodeMap.
	astNode, ok := c.nodeMap[expr]
	if !ok {
		return nil
	}

	// Convert ast.Node to ast.Expr.
	astExpr, ok := astNode.(ast.Expr)
	if !ok {
		return nil
	}

	// Get the type from the typeInfo map.
	if t, ok := c.typeInfo.Types[astExpr]; ok {
		return t.Type
	}

	// For identifiers, try to get the type from Uses.
	if astIdent, ok := astExpr.(*ast.Ident); ok {
		if obj, ok := c.typeInfo.Uses[astIdent]; ok {
			return obj.Type()
		}
	}

	// For selector expressions (pkg.Type), try Uses on the selector.
	if selExpr, ok := astExpr.(*ast.SelectorExpr); ok {
		if obj, ok := c.typeInfo.Uses[selExpr.Sel]; ok {
			return obj.Type()
		}
	}

	// For star expressions (*Type), resolve the underlying type and return a pointer.
	// Need to convert back from ast.Expr to dst.Expr to use nodeMap recursively.
	if starExpr, ok := astExpr.(*ast.StarExpr); ok {
		// Find the corresponding dst.Node for starExpr.X
		var dstX dst.Expr
		// Iterate through nodeMap to find the dst.Node corresponding to ast.Node starExpr.X
		// This is inefficient but necessary as we don't have a direct reverse mapping.
		// Consider optimizing if performance becomes an issue.
		for dNode, aNode := range c.nodeMap {
			if aNode == starExpr.X {
				if dExpr, ok := dNode.(dst.Expr); ok {
					dstX = dExpr
					break
				}
			}
		}

		if dstX != nil {
			underlyingType := c.ResolveType(dstX)
			if underlyingType != nil {
				return types.NewPointer(underlyingType)
			}
		}
	}

	return nil
}
