// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package at

import (
	"fmt"
	"strings"

	"github.com/dave/dst"
	"github.com/dave/dst/dstutil"
	"gopkg.in/yaml.v3"
)

type (
	FuncDeclOption interface {
		evaluate(*dst.FuncDecl) bool
	}

	funcDecl struct {
		opts []FuncDeclOption
	}
)

// FunctionDeclaration matches function declaration nodes based on properties of
// their signature.
func FunctionDeclaration(opts ...FuncDeclOption) *funcDecl {
	return &funcDecl{opts: opts}
}

func (s *funcDecl) Matches(csor *dstutil.Cursor) bool {
	return s.matchesNode(csor.Node())
}

func (s *funcDecl) matchesNode(node dst.Node) bool {
	decl, ok := node.(*dst.FuncDecl)
	if !ok {
		return false
	}

	for _, opt := range s.opts {
		if !opt.evaluate(decl) {
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
func Signature(args []TypeName, ret []TypeName) FuncDeclOption {
	return &signature{args: args, returns: ret}
}

func (fo *signature) evaluate(decl *dst.FuncDecl) bool {
	fnType := decl.Type

	if fnType.Results == nil {
		if len(fo.returns) != 0 {
			return false
		}
	} else if len(fnType.Results.List) != len(fo.returns) {
		return false
	} else {
		for i := 0; i < len(fo.returns); i++ {
			if !fo.returns[i].matches(fnType.Results.List[i].Type) {
				return false
			}
		}
	}

	if fnType.Params == nil {
		if len(fo.args) != 0 {
			return false
		}
	} else if len(fnType.Params.List) != len(fo.args) {
		return false
	} else {
		for i := 0; i < len(fo.args); i++ {
			if !fo.args[i].matches(fnType.Params.List[i].Type) {
				return false
			}
		}
	}

	return true
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

func (s *funcBody) Matches(csor *dstutil.Cursor) bool {
	parentNode := csor.Parent()
	if !s.up.matchesNode(parentNode) {
		return false
	}

	funcDecl := parentNode.(*dst.FuncDecl)
	return csor.Node() == funcDecl.Body
}

func init() {
	unmarshallers["function-body"] = func(node *yaml.Node) (InjectionPoint, error) {
		ip, err := Unmarshal(node)
		if err != nil {
			return nil, err
		}
		up, ok := ip.(*funcDecl)
		if !ok {
			return nil, fmt.Errorf("line %d: function-body only supports function-declaration injection points", node.Content[1].Line)
		}
		return FunctionBody(up), nil
	}

	unmarshallers["function-declaration"] = func(node *yaml.Node) (InjectionPoint, error) {
		var unmarshalOpts []unmarshalFuncDeclOption
		if err := node.Decode(&unmarshalOpts); err != nil {
			return nil, err
		}
		opts := make([]FuncDeclOption, len(unmarshalOpts))
		for i, opt := range unmarshalOpts {
			opts[i] = opt.FuncDeclOption
		}
		return FunctionDeclaration(opts...), nil
	}
}

type unmarshalFuncDeclOption struct {
	FuncDeclOption
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
				if args[i], err = parseTypeName(a); err != nil {
					return err
				}
			}
		}

		var ret []TypeName
		if len(sig.Ret) > 0 {
			ret = make([]TypeName, len(sig.Ret))
			for i, r := range sig.Ret {
				var err error
				if ret[i], err = parseTypeName(r); err != nil {
					return err
				}
			}
		}

		o.FuncDeclOption = Signature(args, ret)
	default:
		return fmt.Errorf("line %d: unknown FuncDeclOption name: %q", node.Content[0].Line, key)
	}

	return nil
}
