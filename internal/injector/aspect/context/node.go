// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package context

import (
	"sync"

	"github.com/dave/dst"
	"github.com/dave/dst/dstutil"
)

type NodeChain struct {
	parent *NodeChain

	node   dst.Node
	config map[string]string
	name   string
	index  int
}

var chainPool = sync.Pool{New: func() any { return new(NodeChain) }}

// Child creates a new [NodeChain] that represents the node pointed to by the provided
// [dstutil.Cursor]. The cursor's parent must match the receiver. Values returned by
// [NodeChain.Child] should be returned to the pool by calling [NodeChain.Release] on them, as this
// allows for a significant reduction of allocations.
func (n *NodeChain) Child(cursor *dstutil.Cursor) *NodeChain {
	if n != nil && n.node != cursor.Parent() {
		panic("cursor does not point to a child of this node")
	}

	c, _ := chainPool.Get().(*NodeChain)
	*c = NodeChain{
		parent: n,
		node:   cursor.Node(),
		name:   cursor.Name(),
		index:  cursor.Index(),
	}

	return c
}

// Release returns the [NodeChain] to the backing pool so that it can be re-used. This signficicantly
// reduces allocations while walking ASTs, as one [NodeChain] is used for each AST node, but they are
// very short-lived.
func (n *NodeChain) Release() {
	*n = NodeChain{} // Zero it off
	chainPool.Put(n)
}

func (n *NodeChain) SetConfig(val map[string]string) {
	n.config = val
}

func (n *NodeChain) Config(name string) (string, bool) {
	for p := n; p != nil; p = p.parent {
		if val, found := p.config[name]; found {
			return val, true
		}
	}
	return "", false
}

func (n *NodeChain) Parent() *NodeChain {
	return n.parent
}

func (n *NodeChain) Node() dst.Node {
	return n.node
}

func (n *NodeChain) PropertyName() string {
	return n.name
}

func (n *NodeChain) Index() int {
	return n.index
}
