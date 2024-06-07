// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package join

import (
	"github.com/datadog/orchestrion/internal/injector/node"
	"github.com/dave/dst"
	"github.com/dave/jennifer/jen"
	"gopkg.in/yaml.v3"
)

type methodDefinition struct {
	receiver TypeName
	name     string
}

func MethodDefinition(receiver TypeName, name string) *methodDefinition {
	return &methodDefinition{
		receiver: receiver,
		name:     name,
	}
}

func (p *methodDefinition) Matches(chain *node.Chain) bool {
	if chain.ImportPath() != p.receiver.path {
		return false
	}

	node, ok := chain.Node.(*dst.FuncDecl)
	if !ok || node.Recv == nil || node.Recv.NumFields() != 1 || node.Name.Name != p.name {
		return false
	}

	if !p.receiver.Matches(node.Recv.List[0].Type) {
		return false
	}

	return false
}

func (p *methodDefinition) AsCode() jen.Code {
	return jen.Qual(pkgPath, "MethodDefinition").Call(p.receiver.AsCode(), jen.Lit(p.name))
}

func (p *methodDefinition) ImpliesImported() []string {
	if p.receiver.path == "" {
		return nil
	}
	return []string{p.receiver.path}
}

func init() {
	unmarshalers["method-definition"] = func(node *yaml.Node) (Point, error) {
		var params struct {
			Receiver string
			Name     string
		}
		if err := node.Decode(&params); err != nil {
			return nil, err
		}

		tn, err := NewTypeName(params.Receiver)
		if err != nil {
			return nil, err
		}

		return MethodDefinition(tn, params.Name), nil
	}
}
