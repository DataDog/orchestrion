// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package context

import (
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

func (n *NodeChain) Child(cursor *dstutil.Cursor) *NodeChain {
	if n != nil && n.node != cursor.Parent() {
		panic("cursor does not point to a child of this node")
	}
	return &NodeChain{
		parent: n,
		node:   cursor.Node(),
		name:   cursor.Name(),
		index:  cursor.Index(),
	}
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

func (n *NodeChain) Index() string {
	return n.Index()
}
