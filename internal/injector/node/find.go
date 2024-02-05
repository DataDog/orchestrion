// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package node

import "github.com/dave/dst"

// Find looks for a node of the specified type T in the provided node Chain,
// and returns the closest ancestor found. If no ancestor is found, the
// zero-value of T is returned with false.
func Find[T dst.Node](chain *Chain) (node T, found bool) {
	for curr := chain; curr != nil; curr = curr.Parent() {
		if node, found = curr.Node.(T); found {
			return
		}
	}
	return
}
