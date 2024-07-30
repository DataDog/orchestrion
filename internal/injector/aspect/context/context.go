// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package context

import (
	"github.com/datadog/orchestrion/internal/injector/typed"
	"github.com/dave/dst"
	"github.com/dave/dst/dstutil"
)

type AspectContext interface {
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
}

type context struct {
	*NodeChain
	cursor *dstutil.Cursor

	// Common to all contexts in the same hierarchy...
	file         *dst.File
	refMap       *typed.ReferenceMap
	sourceParser SourceParser
	importPath   string
}

type SourceParser interface {
	Parse(any) (*dst.File, error)
}

func (n *NodeChain) Context(
	cursor *dstutil.Cursor,
	importPath string,
	file *dst.File,
	refMap *typed.ReferenceMap,
	sourceParser SourceParser,
) *context {
	return &context{
		NodeChain:    n,
		cursor:       cursor,
		file:         file,
		refMap:       refMap,
		sourceParser: sourceParser,
		importPath:   importPath,
	}
}

func (c *context) Child(node dst.Node, property string, index int) AdviceContext {
	return &context{
		NodeChain: &NodeChain{
			parent: c.NodeChain,
			node:   node,
			name:   property,
			index:  index,
		},
		cursor:       nil,
		file:         c.file,
		refMap:       c.refMap,
		sourceParser: c.sourceParser,
		importPath:   c.importPath,
	}
}

func (c *context) Parent() AspectContext {
	parent := c.NodeChain.parent
	if parent == nil {
		return nil
	}

	return &context{
		NodeChain:  parent,
		file:       c.file,
		refMap:     c.refMap,
		importPath: c.importPath,
	}
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

func (c *context) ParseSource(bytes []byte) (*dst.File, error) {
	return c.sourceParser.Parse(bytes)
}

func (c *context) AddImport(path string, alias string) bool {
	return c.refMap.AddImport(c.file, path, alias)
}

func (c *context) AddLink(path string) bool {
	return c.refMap.AddLink(c.file, path)
}
