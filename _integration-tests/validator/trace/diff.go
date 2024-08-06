// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package trace

import (
	"fmt"
	"sort"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/xlab/treeprint"
)

type Diff treeprint.Tree

// RequireAnyMatch asserts that any of the traces in `others` corresponds to the receiver.
func (span *Span) RequireAnyMatch(t *testing.T, others []*Span) {
	span, diff := span.matchesAny(others, treeprint.NewWithRoot("Root"))
	require.NotNil(t, span, "no match found for trace:\n%s", diff)
	t.Logf("Found matching trace:\n%s", span)
}

func (span *Span) matchesAny(others []*Span, diff treeprint.Tree) (*Span, Diff) {
	if len(others) == 0 {
		span.into(diff.AddMetaBranch("-", "No spans to match against"))
		return nil, diff
	}

	for idx, other := range others {
		id := fmt.Sprintf("Span at index %d", idx)
		if other.ID != 0 {
			id = fmt.Sprintf("Span ID %d", other.ID)
		}
		branch := diff.AddMetaBranch("±", id)
		if span.matches(other, branch) {
			return other, nil
		}
	}
	return nil, diff
}

// macthes determines whether the receiving span matches the other span, and
// adds difference information to the provided diff tree.
func (span *Span) matches(other *Span, diff treeprint.Tree) (matches bool) {
	matches = true

	keys := make([]string, 0, len(span.Tags))
	maxLen := 1
	for key := range span.Tags {
		keys = append(keys, key)
		if len := len(key); len > maxLen {
			maxLen = len
		}
	}
	sort.Strings(keys)
	for _, tag := range keys {
		expected := span.Tags[tag]
		actual := other.Tags[tag]
		if expected != actual && (tag != "service" || fmt.Sprintf("%s.exe", expected) != actual) {
			branch := diff.AddMetaBranch("±", tag)
			branch.AddMetaNode("-", expected)
			branch.AddMetaNode("+", actual)
			matches = false
		} else {
			diff.AddMetaNode("=", fmt.Sprintf("%-*s = %q", maxLen, tag, expected))
		}
	}

	keys = make([]string, 0, len(span.Meta))
	maxLen = 1
	for key := range span.Meta {
		keys = append(keys, key)
		if len := len(key); len > maxLen {
			maxLen = len
		}
	}
	sort.Strings(keys)
	var metaNode treeprint.Tree
	for _, key := range keys {
		expected := span.Meta[key]
		actual := other.Meta[key]
		if metaNode == nil {
			metaNode = diff.AddBranch("meta")
		}
		if expected != actual {
			branch := metaNode.AddMetaBranch("±", key)
			branch.AddMetaNode("-", expected)
			branch.AddMetaNode("+", actual)
			matches = false
		} else {
			metaNode.AddMetaNode("=", fmt.Sprintf("%-*s = %q", maxLen, key, expected))
		}
	}

	var childrenNode treeprint.Tree
	for idx, child := range span.Children {
		if childrenNode == nil {
			childrenNode = diff.AddBranch("_children")
		}
		nodeName := fmt.Sprintf("At index %d", idx)
		if len(other.Children) == 0 {
			child.into(childrenNode.AddMetaBranch("-", fmt.Sprintf("%s (no children to match from)", nodeName)))
			matches = false
			continue
		}

		if span, childDiff := child.matchesAny(other.Children, treeprint.New()); span != nil {
			if span.ID != 0 {
				nodeName = fmt.Sprintf("Span #%d", span.ID)
			}
			child.into(childrenNode.AddMetaBranch("=", nodeName))
		} else {
			childDiff.SetMetaValue("±")
			childDiff.SetValue(nodeName)
			childrenNode.AddNode(childDiff)
			matches = false
		}
	}

	return
}
