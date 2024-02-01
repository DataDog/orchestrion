// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package join

import (
	"github.com/dave/dst"
	"github.com/dave/dst/dstutil"
	"gopkg.in/yaml.v3"
)

type structLiteral struct {
	typeName TypeName
	field    string
}

func StructLiteral(typeName TypeName, field string) *structLiteral {
	return &structLiteral{
		typeName: typeName,
		field:    field,
	}
}

func (s *structLiteral) Matches(csor *dstutil.Cursor) bool {
	return s.matchesNode(csor.Node(), csor.Parent())
}

func (s *structLiteral) matchesNode(node dst.Node, parent dst.Node) bool {
	if s.field == "" {
		return s.matchesLiteral(node)
	}

	kve, ok := node.(*dst.KeyValueExpr)
	if !ok {
		return false
	}

	if !s.matchesLiteral(parent) {
		return false
	}

	key, ok := kve.Key.(*dst.Ident)
	if !ok {
		return false
	}

	return key.Name == s.field
}

func (s *structLiteral) matchesLiteral(node dst.Node) bool {
	lit, ok := node.(*dst.CompositeLit)
	if !ok {
		return false
	}
	return s.typeName.matches(lit.Type)
}

func init() {
	unmarshalers["struct-literal"] = func(node *yaml.Node) (Point, error) {
		var spec struct {
			Type  string
			Field string
		}
		if err := node.Decode(&spec); err != nil {
			return nil, err
		}

		tn, err := parseTypeName(spec.Type)
		if err != nil {
			return nil, err
		}

		return StructLiteral(tn, spec.Field), nil
	}
}
