// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package join

import (
	"fmt"
	"strings"

	"github.com/DataDog/orchestrion/internal/injector/aspect/context"
	"github.com/DataDog/orchestrion/internal/injector/code"
	"github.com/dave/dst"
	"github.com/dave/jennifer/jen"
	"gopkg.in/yaml.v3"
)

type (
	functionInformation struct {
		Receiver   dst.Expr      // The receiver if this is a method declaration
		Type       *dst.FuncType // The function's type signature
		ImportPath string        // The import path of the package containing the function
		Name       string        // The name of the function (blank for function literal expressions)
	}

	FunctionOption interface {
		code.AsCode
		impliesImported() []string
		evaluate(functionInformation) bool
		toHTML() string
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

func (s *funcDecl) ImpliesImported() (list []string) {
	for _, opt := range s.opts {
		list = append(list, opt.impliesImported()...)
	}
	return
}

func (s *funcDecl) Matches(ctx context.AspectContext) bool {
	info := functionInformation{
		ImportPath: ctx.ImportPath(),
	}

	if decl, ok := ctx.Node().(*dst.FuncDecl); ok {
		if decl.Recv != nil && len(decl.Recv.List) == 1 {
			info.Receiver = decl.Recv.List[0].Type
		}
		info.Name = decl.Name.Name
		info.Type = decl.Type
	} else if lit, ok := ctx.Node().(*dst.FuncLit); ok {
		info.Type = lit.Type
	} else {
		return false
	}

	for _, opt := range s.opts {
		if !opt.evaluate(info) {
			return false
		}
	}

	return true
}

func (s *funcDecl) AsCode() jen.Code {
	return jen.Qual(pkgPath, "Function").CallFunc(func(g *jen.Group) {
		for _, opt := range s.opts {
			g.Line().Add(opt.AsCode())
		}
		g.Empty().Line()
	})
}

func (s *funcDecl) RenderHTML() string {
	var buf strings.Builder

	_, _ = buf.WriteString("<div class=\"join-point function-declaratop,\">\n")
	_, _ = buf.WriteString("  <span class=\"type pill\">Function declaration</span>\n")
	_, _ = buf.WriteString("  <ul>\n")
	for _, opt := range s.opts {
		_, _ = buf.WriteString("    <li>\n")
		_, _ = buf.WriteString(opt.toHTML())
		_, _ = buf.WriteString("    </li>\n")
	}
	_, _ = buf.WriteString("  </ul>\n")
	_, _ = buf.WriteString("</div>\n")

	return buf.String()
}

type funcName string

func Name(name string) FunctionOption {
	return funcName(name)
}

func (funcName) impliesImported() []string {
	return nil
}

func (fo funcName) evaluate(info functionInformation) bool {
	return info.Name == string(fo)
}

func (fo funcName) AsCode() jen.Code {
	return jen.Qual(pkgPath, "Name").Call(jen.Lit(string(fo)))
}

func (fo funcName) toHTML() string {
	if fo == "" {
		return "<div class=\"join-point function-option fo-name\"><span class=\"type pill\">Function literal expression</span></div>"
	}
	return fmt.Sprintf("<div class=\"join-point flex function-option fo-name\"><span class=\"type\">Function name</span><code>%s</code></div>", string(fo))
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

func (fo *signature) impliesImported() (list []string) {
	for _, tn := range fo.args {
		if path := tn.ImportPath(); path != "" {
			list = append(list, path)
		}
	}
	for _, tn := range fo.returns {
		if path := tn.ImportPath(); path != "" {
			list = append(list, path)
		}
	}
	return
}

func (fo *signature) evaluate(info functionInformation) bool {
	if info.Type.Results == nil || len(info.Type.Results.List) == 0 {
		if len(fo.returns) != 0 {
			return false
		}
	} else if len(info.Type.Results.List) != len(fo.returns) {
		return false
	} else {
		for i := 0; i < len(fo.returns); i++ {
			if !fo.returns[i].Matches(info.Type.Results.List[i].Type) {
				return false
			}
		}
	}

	if info.Type.Params == nil || len(info.Type.Params.List) == 0 {
		if len(fo.args) != 0 {
			return false
		}
	} else if len(info.Type.Params.List) != len(fo.args) {
		return false
	} else {
		for i := 0; i < len(fo.args); i++ {
			if !fo.args[i].Matches(info.Type.Params.List[i].Type) {
				return false
			}
		}
	}

	return true
}

func (fo *signature) AsCode() jen.Code {
	return jen.Qual(pkgPath, "Signature").CallFunc(func(g *jen.Group) {
		if len(fo.args) > 0 {
			g.Line().Index().Qual(pkgPath, "TypeName").ValuesFunc(func(g *jen.Group) {
				for _, arg := range fo.args {
					g.Add(arg.AsCode())
				}
			})
		} else {
			g.Line().Nil()
		}
		if len(fo.returns) > 0 {
			g.Line().Index().Qual(pkgPath, "TypeName").ValuesFunc(func(g *jen.Group) {
				for _, ret := range fo.returns {
					g.Add(ret.AsCode())
				}
			})
		} else {
			g.Line().Nil()
		}
		g.Empty().Line()
	})
}

func (fo *signature) toHTML() string {
	var buf strings.Builder

	_, _ = buf.WriteString("<div class=\"join-point function-option fo-signature\">\n")
	_, _ = buf.WriteString("  <span class=\"type pill\">Signature matches</span>\n")
	_, _ = buf.WriteString("<ul>\n")

	if len(fo.args) > 0 {
		_, _ = buf.WriteString("    <li>\n")
		_, _ = buf.WriteString("      <span class=\"type pill\">Arguments</span>\n")
		_, _ = buf.WriteString("      <ol>\n")
		for _, arg := range fo.args {
			_, _ = buf.WriteString("        <li class=\"flex\"><span class=\"id\"></span>\n")
			_, _ = buf.WriteString(arg.RenderHTML())
			_, _ = buf.WriteString("        </li>\n")
		}
		_, _ = buf.WriteString("      </ol>\n")
		_, _ = buf.WriteString("    </li>\n")
	} else {
		_, _ = buf.WriteString("    <li class=\"flex\"><span class=\"type\">Arguments</span><span class=\"value\">None</span></li>\n")
	}

	if len(fo.returns) > 0 {
		_, _ = buf.WriteString("    <li>\n")
		_, _ = buf.WriteString("      <span class=\"type pill\">Return Values</span>\n")
		_, _ = buf.WriteString("      <ol>\n")
		for _, arg := range fo.returns {
			_, _ = buf.WriteString("        <li class=\"flex\"><span class=\"id\"></span>\n")
			_, _ = buf.WriteString(arg.RenderHTML())
			_, _ = buf.WriteString("        </li>\n")
		}
		_, _ = buf.WriteString("      </ol>\n")
		_, _ = buf.WriteString("    </li>\n")
	} else {
		_, _ = buf.WriteString("    <li class=\"flex\"><span class=\"type\">Return Values</span><span class=\"value\">None</span></li>\n")
	}

	_, _ = buf.WriteString("</ul>\n")
	_, _ = buf.WriteString("</div>\n")

	return buf.String()
}

type oneOfFunctions []FunctionOption

func OneOfFunctions(opts ...FunctionOption) oneOfFunctions {
	return oneOfFunctions(opts)
}

func (fo oneOfFunctions) impliesImported() (list []string) {
	// We can only assume a package is imported if all candidates imply it.
	counts := make(map[string]uint)
	for _, opt := range fo {
		for _, path := range opt.impliesImported() {
			counts[path]++
		}
	}

	total := uint(len(fo))
	list = make([]string, 0, len(counts))
	for path, count := range counts {
		if count == total {
			list = append(list, path)
		}
	}
	return
}

func (fo oneOfFunctions) evaluate(info functionInformation) bool {
	for _, opt := range fo {
		if opt.evaluate(info) {
			return true
		}
	}
	return false
}

func (fo oneOfFunctions) AsCode() jen.Code {
	if len(fo) == 1 {
		return (fo)[0].AsCode()
	}

	return jen.Qual(pkgPath, "OneOfFunctions").CallFunc(func(g *jen.Group) {
		for _, opt := range fo {
			g.Line().Add(opt.AsCode())
		}
		g.Line().Empty()
	})
}

func (oneOfFunctions) toHTML() string {
	return "one-of"
}

type receives struct {
	typeName TypeName
}

func Receives(typeName TypeName) FunctionOption {
	return &receives{typeName}
}

func (fo *receives) impliesImported() []string {
	if path := fo.typeName.ImportPath(); path != "" {
		return []string{path}
	}
	return nil
}

func (fo *receives) evaluate(info functionInformation) bool {
	for _, param := range info.Type.Params.List {
		if fo.typeName.Matches(param.Type) {
			return true
		}
	}
	return false
}

func (fo *receives) AsCode() jen.Code {
	return jen.Qual(pkgPath, "Receives").Call(fo.typeName.AsCode())
}

func (fo *receives) toHTML() string {
	return fmt.Sprintf(`<div class="flex join-point function-option fo-receives"><span class="type">Has parameter</span>%s</div>`, fo.typeName.RenderHTML())
}

type receiver struct {
	typeName TypeName
}

func Receiver(typeName TypeName) FunctionOption {
	return &receiver{typeName}
}

func (fo *receiver) evaluate(info functionInformation) bool {
	return info.Receiver != nil && fo.typeName.MatchesDefinition(info.Receiver, info.ImportPath)
}

func (*receiver) impliesImported() []string {
	return nil
}

func (fo *receiver) AsCode() jen.Code {
	return jen.Qual(pkgPath, "Receiver").Call(fo.typeName.AsCode())
}

func (fo *receiver) toHTML() string {
	return fmt.Sprintf(`<div class="flex join-point function-option fo-receiver"><span class="type">Is method of</span>%s</div>`, fo.typeName.RenderHTML())
}

type funcBody struct {
	up Point
}

// FunctionBody returns the *dst.BlockStmt of the matched *dst.FuncDecl body.
func FunctionBody(up Point) *funcBody {
	if up == nil {
		panic("upstream FunctionDeclaration InjectionPoint cannot be nil")
	}
	return &funcBody{up: up}
}

func (s *funcBody) ImpliesImported() []string {
	return s.up.ImpliesImported()
}

func (s *funcBody) Matches(ctx context.AspectContext) bool {
	if parent := ctx.Parent(); parent == nil || !s.up.Matches(parent) {
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

func (s *funcBody) AsCode() jen.Code {
	return jen.Qual(pkgPath, "FunctionBody").Call(s.up.AsCode())
}

func (s *funcBody) RenderHTML() string {
	return fmt.Sprintf(`<div class="join-point function-body"><span class="type pill">Function body</span><ul><li>%s</li></ul></div>`, s.up.RenderHTML())
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
	case "one-of":
		var opts []unmarshalFuncDeclOption
		if err := node.Content[1].Decode(&opts); err != nil {
			return err
		}
		matchers := make([]FunctionOption, len(opts))
		for i, opt := range opts {
			matchers[i] = opt.FunctionOption
		}
		o.FunctionOption = OneOfFunctions(matchers...)
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
			Extra map[string]yaml.Node `yaml:",inline"`
			Args  []string             `yaml:"args"`
			Ret   []string             `yaml:"returns"`
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
