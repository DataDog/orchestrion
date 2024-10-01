// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package join

import (
	"fmt"
	"strings"

	"github.com/DataDog/orchestrion/internal/injector/aspect/context"
	"github.com/dave/dst"
	"github.com/dave/jennifer/jen"
	"gopkg.in/yaml.v3"
)

type (
	functionInformation struct {
		ImportPath  string          // The import path of the package containing the function
		Receiver    dst.Expr        // The receiver if this is a method declaration
		Name        string          // The name of the function (blank for function literal expressions)
		Type        *dst.FuncType   // The function's type signature
		Decorations []*dst.NodeDecs // The function's decoration chain
	}

	FunctionOption interface {
		// asCode produces a jen.Code representation of the receiver.
		asCode() jen.Code

		impliesImported() []string
		evaluate(*functionInformation) bool
	}

	functionDeclaration struct {
		Options []FunctionOption
	}
)

// Function matches function declaration nodes based on properties of
// their signature.
func Function(opts ...FunctionOption) *functionDeclaration {
	return &functionDeclaration{Options: opts}
}

func (s *functionDeclaration) ImpliesImported() (list []string) {
	for _, opt := range s.Options {
		list = append(list, opt.impliesImported()...)
	}
	return
}

func (s *functionDeclaration) Matches(ctx context.AspectContext) bool {
	info := functionInformation{
		ImportPath:  ctx.ImportPath(),
		Decorations: []*dst.NodeDecs{ctx.Node().Decorations()},
	}

	if decl, ok := ctx.Node().(*dst.FuncDecl); ok {
		if decl.Recv != nil && len(decl.Recv.List) == 1 {
			info.Receiver = decl.Recv.List[0].Type
		}
		info.Name = decl.Name.Name
		info.Type = decl.Type
	} else if lit, ok := ctx.Node().(*dst.FuncLit); ok {
		info.Type = lit.Type
		if parent := ctx.Parent(); parent != nil {
			if parent, ok := parent.Node().(*dst.AssignStmt); ok {
				info.Decorations = append(info.Decorations, parent.Decorations())
			}
		}
	} else {
		return false
	}

	for _, opt := range s.Options {
		if !opt.evaluate(&info) {
			return false
		}
	}

	return true
}

func (s *functionDeclaration) AsCode() jen.Code {
	return jen.Qual(pkgPath, "Function").CallFunc(func(g *jen.Group) {
		for _, opt := range s.Options {
			g.Line().Add(opt.asCode())
		}
		g.Empty().Line()
	})
}

type functionName string

func Name(name string) FunctionOption {
	return functionName(name)
}

func (functionName) impliesImported() []string {
	return nil
}

func (fo functionName) evaluate(info *functionInformation) bool {
	return info.Name == string(fo)
}

func (fo functionName) asCode() jen.Code {
	return jen.Qual(pkgPath, "Name").Call(jen.Lit(string(fo)))
}

type signature struct {
	Arguments []TypeName
	Results   []TypeName
}

// Signature matches function declarations based on their arguments and return
// value types.
func Signature(args []TypeName, ret []TypeName) FunctionOption {
	return &signature{Arguments: args, Results: ret}
}

func (fo *signature) impliesImported() (list []string) {
	for _, tn := range fo.Arguments {
		if path := tn.ImportPath(); path != "" {
			list = append(list, path)
		}
	}
	for _, tn := range fo.Results {
		if path := tn.ImportPath(); path != "" {
			list = append(list, path)
		}
	}
	return
}

func (fo *signature) evaluate(info *functionInformation) bool {
	if info.Type.Results == nil || len(info.Type.Results.List) == 0 {
		if len(fo.Results) != 0 {
			return false
		}
	} else if len(info.Type.Results.List) != len(fo.Results) {
		return false
	} else {
		for i := 0; i < len(fo.Results); i++ {
			if !fo.Results[i].Matches(info.Type.Results.List[i].Type) {
				return false
			}
		}
	}

	if info.Type.Params == nil || len(info.Type.Params.List) == 0 {
		if len(fo.Arguments) != 0 {
			return false
		}
	} else if len(info.Type.Params.List) != len(fo.Arguments) {
		return false
	} else {
		for i := 0; i < len(fo.Arguments); i++ {
			if !fo.Arguments[i].Matches(info.Type.Params.List[i].Type) {
				return false
			}
		}
	}

	return true
}

func (fo *signature) asCode() jen.Code {
	return jen.Qual(pkgPath, "Signature").CallFunc(func(g *jen.Group) {
		if len(fo.Arguments) > 0 {
			g.Line().Index().Qual(pkgPath, "TypeName").ValuesFunc(func(g *jen.Group) {
				for _, arg := range fo.Arguments {
					g.Add(arg.AsCode())
				}
			})
		} else {
			g.Line().Nil()
		}
		if len(fo.Results) > 0 {
			g.Line().Index().Qual(pkgPath, "TypeName").ValuesFunc(func(g *jen.Group) {
				for _, ret := range fo.Results {
					g.Add(ret.AsCode())
				}
			})
		} else {
			g.Line().Nil()
		}
		g.Empty().Line()
	})
}

type receiver struct {
	TypeName TypeName
}

func Receiver(typeName TypeName) FunctionOption {
	return &receiver{typeName}
}

func (fo *receiver) evaluate(info *functionInformation) bool {
	return info.Receiver != nil && fo.TypeName.MatchesDefinition(info.Receiver, info.ImportPath)
}

func (*receiver) impliesImported() []string {
	return nil
}

func (fo *receiver) asCode() jen.Code {
	return jen.Qual(pkgPath, "Receiver").Call(fo.TypeName.AsCode())
}

type functionBody struct {
	Function Point
}

// FunctionBody returns the *dst.BlockStmt of the matched *dst.FuncDecl body.
func FunctionBody(up Point) *functionBody {
	if up == nil {
		panic("upstream FunctionDeclaration InjectionPoint cannot be nil")
	}
	return &functionBody{Function: up}
}

func (s *functionBody) ImpliesImported() []string {
	return s.Function.ImpliesImported()
}

func (s *functionBody) Matches(ctx context.AspectContext) bool {
	if parent := ctx.Parent(); parent == nil || !s.Function.Matches(parent) {
		return false
	}

	switch parent := ctx.Parent().Node().(type) {
	case *dst.FuncDecl:
		return ctx.Node() == parent.Body
	case *dst.FuncLit:
		return ctx.Node() == parent.Body
	default:
		return false
	}
}

func (s *functionBody) AsCode() jen.Code {
	return jen.Qual(pkgPath, "FunctionBody").Call(s.Function.AsCode())
}

func init() {
	unmarshalers["function-body"] = func(node *yaml.Node) (Point, error) {
		up, err := FromYAML(node)
		if err != nil {
			return nil, err
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
	case "name":
		var name string
		if err := node.Content[1].Decode(&name); err != nil {
			return err
		}
		o.FunctionOption = Name(name)
	case "receiver":
		var arg string
		if err := node.Content[1].Decode(&arg); err != nil {
			return err
		}
		tn, err := NewTypeName(arg)
		if err != nil {
			return err
		}
		o.FunctionOption = Receiver(tn)
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
