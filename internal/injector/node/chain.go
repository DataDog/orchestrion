// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package node

import (
	"errors"
	"fmt"

	"github.com/dave/dst"
	"github.com/dave/dst/dstutil"
)

// Chain represents a chain of nodes, where the tip node is associated to
// all its ancestors up to the root node.
type Chain struct {
	dst.Node                   // The node which ancestry is tracked by this
	parent   *Chain            // The ancestor of this node
	config   map[string]string // The node's own configuration
	name     string            // The name of this node according to its parent (or an empty string)
	index    int               // The index of this node in the list it belonds to (or -1 if it's not in a list)

	repr string
}

// Child creates a new NodeChain with the current node as the parent of the new
// node, using the specified Name and Index values. This is safe to call on a
// nil receiver.
func (nc *Chain) Child(node dst.Node, name string, index int) *Chain {
	return &Chain{Node: node, parent: nc, name: name, index: index}
}

// ChildFromCursor creates a new NodeChain with the current node as the parent
// of the new node, which is populated from the cursor. This method is safe to
// call on a nil receiver. Panics if the receiver is not nil, and the
// *dstutil.Cursor reports a different parent node.
func (nc *Chain) ChildFromCursor(csor *dstutil.Cursor) *Chain {
	if nc != nil && nc.Node != csor.Parent() {
		panic(errors.New("attempted to create a NodeChain that does not match reality"))
	}
	return nc.Child(csor.Node(), csor.Name(), csor.Index())
}

// Parent returns the parent of this node, or nil if this node is the root.
func (nc *Chain) Parent() *Chain {
	if nc == nil {
		return nil
	}
	return nc.parent
}

// Name returns the name of the field in the parent node that contains this node,
// or an empty string if this node does not have a parent or is not from a field.
func (nc *Chain) Name() string {
	if nc == nil {
		return ""
	}
	return nc.name
}

// Index returns the index of this node in the collection it belongs to, or a
// negative value if this node is not part of a collection.
func (nc *Chain) Index() int {
	if nc == nil {
		return -1
	}
	return nc.index
}

// SetConfig sets the given configuration key-value pair on this node. This will
// overwrite any existing value for the same key, and shadows any value for the
// same key in the node's ancestry.
func (nc *Chain) SetConfig(key, value string) {
	if nc.config == nil {
		nc.config = map[string]string{key: value}
		return
	}
	nc.config[key] = value
}

// Config reads the value of the given configuration key from this node or any
// of its ancestors, whichever returns a value first. If no value is found, an
// emty string and false are returned.
func (nc *Chain) Config(key string) (value string, found bool) {
	value, found = nc.config[key]
	if !found && nc.parent != nil {
		value, found = nc.parent.Config(key)
	}
	return
}

func (nc *Chain) String() string {
	if nc == nil {
		return "<nil>"
	}

	if nc.repr == "" {
		if parent := nc.Parent(); parent != nil {
			nc.repr = parent.String() + " > "
		}
		nc.repr += fmt.Sprintf("%T", nc.Node)
	}
	return nc.repr
}

// As attempts to convert the provided Chain into a dst.Node of the specified
// sub-type.
func As[T dst.Node](nc *Chain) (cast T, ok bool) {
	cast, ok = nc.Node.(T)
	return
}
