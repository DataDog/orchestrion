// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package trace

import (
	"encoding/json"
	"fmt"
	"sort"
	"strconv"

	"github.com/xlab/treeprint"
)

type SpanID uint64

// Span represents a span within a trace, which is hierarchically organized
// via the Children property.
type Span struct {
	ID       SpanID `json:"span_id"`
	Meta     map[string]string
	Metrics  map[string]float64
	Tags     map[string]any
	Children []*Span
}

type Spans = []*Span

var _ json.Unmarshaler = &Span{}

func (span *Span) UnmarshalJSON(data []byte) error {
	span.Meta = nil
	span.Tags = make(map[string]any)
	span.Children = nil

	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}

	for key, value := range raw {
		var err error
		switch key {
		case "_children":
			err = json.Unmarshal(value, &span.Children)
		case "meta":
			err = json.Unmarshal(value, &span.Meta)
		case "metrics":
			err = json.Unmarshal(value, &span.Metrics)
		case "span_id":
			err = json.Unmarshal(value, &span.ID)
			if err == nil {
				span.Tags["span_id"] = json.Number(fmt.Sprintf("%d", span.ID))
			}
		default:
			var val any
			err = json.Unmarshal(value, &val)
			span.Tags[key] = val
		}
		if err != nil {
			return err
		}
	}

	return nil
}

func (span *Span) String() string {
	tree := treeprint.NewWithRoot("Root")
	span.into(tree)
	return tree.String()
}

func (span *Span) into(tree treeprint.Tree) {
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
		tree.AddNode(fmt.Sprintf("%-*s = %q", maxLen, tag, span.Tags[tag]))
	}

	addMapBranch(tree, span.Meta, "meta")
	addMapBranch(tree, span.Metrics, "metrics")

	if len(span.Children) > 0 {
		children := tree.AddBranch("_children")
		for i, child := range span.Children {
			child.into(children.AddBranch(fmt.Sprintf("#%d", i)))
		}
	}
}

func addMapBranch[T string | float64](tree treeprint.Tree, m map[string]T, name string) {
	if len(m) > 0 {
		keys := make([]string, 0, len(m))
		maxLen := 1
		for key := range m {
			keys = append(keys, key)
			if l := len(key); l > maxLen {
				maxLen = l
			}
		}
		sort.Strings(keys)
		br := tree.AddBranch(name)
		for _, key := range keys {
			val := m[key]
			printVal := ""
			switch v := any(val).(type) {
			case string:
				printVal = fmt.Sprintf("%q", v)
			case float64:
				printVal = strconv.FormatFloat(v, 'f', -1, 64)
			}
			br.AddNode(fmt.Sprintf("%-*s = %s", maxLen, key, printVal))
		}
	}
}
