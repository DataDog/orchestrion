// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package join

import (
	"fmt"
	"strings"

	"github.com/datadog/orchestrion/internal/injector/node"
	"github.com/dave/dst"
	"gopkg.in/yaml.v3"
)

type (
	FunctionOption interface {
		evaluate(*dst.FuncType, ...*dst.NodeDecs) bool
	}

	funcDecl struct {
		opts []FunctionOption
	}
)

// Function matches function declaration nodes based on properties of
// their signature.
func Function(opts ...FunctionOption) *funcDecl {
	return &funcDecl{opts: opts}
}

func (s *funcDecl) Matches(chain *node.Chain) bool {
	var funcType *dst.FuncType
	funcDecs := []*dst.NodeDecs{chain.Decorations(), nil}[0:1]

	if decl, ok := node.As[*dst.FuncDecl](chain); ok {
		funcType = decl.Type
	} else if lit, ok := node.As[*dst.FuncLit](chain); ok {
		funcType = lit.Type
		if parent, ok := node.As[*dst.AssignStmt](chain.Parent()); ok {
			funcDecs = append(funcDecs, parent.Decorations())
		}
	} else {
		return false
	}

	for _, opt := range s.opts {
		if !opt.evaluate(funcType, funcDecs...) {
			return false
		}
	}

	return true
}

type signature struct {
	args    []TypeName
	returns []TypeName
}

// Signature matches function declarations based on their arguments and return
// value types.
func Signature(args []TypeName, ret []TypeName) FunctionOption {
	return &signature{args: args, returns: ret}
}

func (fo *signature) evaluate(fnType *dst.FuncType, _ ...*dst.NodeDecs) bool {
	if fnType.Results == nil || len(fnType.Results.List) == 0 {
		if len(fo.returns) != 0 {
			return false
		}
	} else if len(fnType.Results.List) != len(fo.returns) {
		return false
	} else {
		for i := 0; i < len(fo.returns); i++ {
			if !fo.returns[i].Matches(fnType.Results.List[i].Type) {
				return false
			}
		}
	}

	if fnType.Params == nil || len(fnType.Params.List) == 0 {
		if len(fo.args) != 0 {
			return false
		}
	} else if len(fnType.Params.List) != len(fo.args) {
		return false
	} else {
		for i := 0; i < len(fo.args); i++ {
			if !fo.args[i].Matches(fnType.Params.List[i].Type) {
				return false
			}
		}
	}

	return true
}

type directive struct {
	name string
}

// Directive matches function declarations based on the presence of a leading
// directive comment.
func Directive(name string) FunctionOption {
	return &directive{name}
}

func (fo *directive) evaluate(_ *dst.FuncType, allDecs ...*dst.NodeDecs) bool {
	for _, decs := range allDecs {
		for _, dec := range decs.Start {
			if dec == "//"+fo.name || strings.HasPrefix(dec, "//"+fo.name+" ") {
				return true
			}
		}
	}
	return false
}

type receives struct {
	typeName TypeName
}

func Receives(typeName TypeName) FunctionOption {
	return &receives{typeName}
}

func (fo *receives) evaluate(fnType *dst.FuncType, _ ...*dst.NodeDecs) bool {
	for _, param := range fnType.Params.List {
		if fo.typeName.Matches(param.Type) {
			return true
		}
	}
	return false
}

type funcBody struct {
	up *funcDecl
}

// FunctionBody returns the *dst.BlockStmt of the matched *dst.FuncDecl body.
func FunctionBody(up *funcDecl) *funcBody {
	if up == nil {
		panic("upstream FunctionDeclaration InjectionPoint cannot be nil")
	}
	return &funcBody{up: up}
}

func (s *funcBody) Matches(chain *node.Chain) bool {
	if parent := chain.Parent(); parent == nil || !s.up.Matches(parent) {
		return false
	}

	switch parent := chain.Parent().Node.(type) {
	case *dst.FuncDecl:
		return chain.Node == parent.Body
	case *dst.FuncLit:
		return chain.Node == parent.Body
	default:
		return false
	}
}

func init() {
	unmarshalers["function-body"] = func(node *yaml.Node) (Point, error) {
		ip, err := FromYAML(node)
		if err != nil {
			return nil, err
		}
		up, ok := ip.(*funcDecl)
		if !ok {
			return nil, fmt.Errorf("line %d: function-body only supports function injection points", node.Content[1].Line)
		}
		return FunctionBody(up), nil
	}

	unmarshalers["function"] = func(node *yaml.Node) (Point, error) {
		var unmarshalOpts []unmarshalFuncDeclOption
		if err := node.Decode(&unmarshalOpts); err != nil {
			return nil, err
		}
		opts := make([]FunctionOption, len(unmarshalOpts))
		for i, opt := range unmarshalOpts {
			opts[i] = opt.FunctionOption
		}
		return Function(opts...), nil
	}
}

type unmarshalFuncDeclOption struct {
	FunctionOption
}

func (o *unmarshalFuncDeclOption) UnmarshalYAML(node *yaml.Node) error {
	if node.Kind != yaml.MappingNode {
		return fmt.Errorf("line %d: cannot unmarshal into a FuncDeclOption: not a mapping", node.Line)
	}

	if len(node.Content) != 2 {
		return fmt.Errorf("line %d: cannot unmarshal into a FuncDeclOption: not a singleton mapping", node.Line)
	}

	var key string
	if err := node.Content[0].Decode(&key); err != nil {
		return err
	}

	switch key {
	case "directive":
		var name string
		if err := node.Content[1].Decode(&name); err != nil {
			return err
		}
		o.FunctionOption = Directive(name)
	case "receives":
		var arg string
		if err := node.Content[1].Decode(&arg); err != nil {
			return err
		}
		tn, err := NewTypeName(arg)
		if err != nil {
			return err
		}
		o.FunctionOption = Receives(tn)
	case "signature":
		var sig struct {
			Args  []string             `yaml:"args"`
			Ret   []string             `yaml:"returns"`
			Extra map[string]yaml.Node `yaml:",inline"`
		}
		if err := node.Content[1].Decode(&sig); err != nil {
			return err
		}
		if len(sig.Extra) != 0 {
			keys := make([]string, 0, len(sig.Extra))
			for key, val := range sig.Extra {
				keys = append(keys, fmt.Sprintf("%q (line %d)", key, val.Line))
			}
			return fmt.Errorf("unexpected keys: %s", strings.Join(keys, ", "))
		}

		var args []TypeName
		if len(sig.Args) > 0 {
			args = make([]TypeName, len(sig.Args))
			for i, a := range sig.Args {
				var err error
				if args[i], err = NewTypeName(a); err != nil {
					return err
				}
			}
		}

		var ret []TypeName
		if len(sig.Ret) > 0 {
			ret = make([]TypeName, len(sig.Ret))
			for i, r := range sig.Ret {
				var err error
				if ret[i], err = NewTypeName(r); err != nil {
					return err
				}
			}
		}

		o.FunctionOption = Signature(args, ret)
	default:
		return fmt.Errorf("line %d: unknown FuncDeclOption name: %q", node.Content[0].Line, key)
	}

	return nil
}
