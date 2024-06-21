// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package join

import (
	"fmt"
	"strings"

	"github.com/datadog/orchestrion/internal/injector/code"
	"github.com/datadog/orchestrion/internal/injector/node"
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
		code.AsCode
		impliesImported() []string
		evaluate(*functionInformation) bool
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

func (s *funcDecl) Matches(chain *node.Chain) bool {
	info := functionInformation{
		ImportPath:  chain.ImportPath(),
		Decorations: []*dst.NodeDecs{chain.Decorations()},
	}

	if decl, ok := node.As[*dst.FuncDecl](chain); ok {
		if decl.Recv != nil && len(decl.Recv.List) == 1 {
			info.Receiver = decl.Recv.List[0].Type
		}
		info.Name = decl.Name.Name
		info.Type = decl.Type
	} else if lit, ok := node.As[*dst.FuncLit](chain); ok {
		info.Type = lit.Type
		if parent, ok := node.As[*dst.AssignStmt](chain.Parent()); ok {
			info.Decorations = append(info.Decorations, parent.Decorations())
		}
	} else {
		return false
	}

	for _, opt := range s.opts {
		if !opt.evaluate(&info) {
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

	buf.WriteString("<div class=\"join-point function-declaratop,\">\n")
	buf.WriteString("  <span class=\"type pill\">Function declaration</span>\n")
	buf.WriteString("  <ul>\n")
	for _, opt := range s.opts {
		buf.WriteString("    <li>\n")
		buf.WriteString(opt.toHTML())
		buf.WriteString("    </li>\n")

	}
	buf.WriteString("  </ul>\n")
	buf.WriteString("</div>\n")

	return buf.String()
}

type funcName string

func Name(name string) FunctionOption {
	return funcName(name)
}

func (fo funcName) impliesImported() []string {
	return nil
}

func (fo funcName) evaluate(info *functionInformation) bool {
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

func (fo *signature) evaluate(info *functionInformation) bool {
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

	buf.WriteString("<div class=\"join-point function-option fo-signature\">\n")
	buf.WriteString("  <span class=\"type pill\">Signature matches</span>\n")
	buf.WriteString("<ul>\n")

	if len(fo.args) > 0 {
		buf.WriteString("    <li>\n")
		buf.WriteString("      <span class=\"type pill\">Arguments</span>\n")
		buf.WriteString("      <ol>\n")
		for _, arg := range fo.args {
			buf.WriteString("        <li class=\"flex\"><span class=\"id\"></span>\n")
			buf.WriteString(arg.RenderHTML())
			buf.WriteString("        </li>\n")
		}
		buf.WriteString("      </ol>\n")
		buf.WriteString("    </li>\n")
	} else {
		buf.WriteString("    <li class=\"flex\"><span class=\"type\">Arguments</span><span class=\"value\">None</span></li>\n")
	}

	if len(fo.returns) > 0 {
		buf.WriteString("    <li>\n")
		buf.WriteString("      <span class=\"type pill\">Return Values</span>\n")
		buf.WriteString("      <ol>\n")
		for _, arg := range fo.returns {
			buf.WriteString("        <li class=\"flex\"><span class=\"id\"></span>\n")
			buf.WriteString(arg.RenderHTML())
			buf.WriteString("        </li>\n")
		}
		buf.WriteString("      </ol>\n")
		buf.WriteString("    </li>\n")
	} else {
		buf.WriteString("    <li class=\"flex\"><span class=\"type\">Return Values</span><span class=\"value\">None</span></li>\n")
	}

	buf.WriteString("</ul>\n")
	buf.WriteString("</div>\n")

	return buf.String()
}

type directive struct {
	name string
}

// Directive matches function declarations based on the presence of a leading
// directive comment.
func Directive(name string) FunctionOption {
	return &directive{name}
}

func (*directive) impliesImported() []string {
	return nil
}

func (fo *directive) evaluate(info *functionInformation) bool {
	for _, decs := range info.Decorations {
		for _, dec := range decs.Start {
			if dec == "//"+fo.name || strings.HasPrefix(dec, "//"+fo.name+" ") {
				return true
			}
		}
	}
	return false
}

func (fo *directive) AsCode() jen.Code {
	return jen.Qual(pkgPath, "Directive").Call(jen.Lit(fo.name))
}

func (fo *directive) toHTML() string {
	return fmt.Sprintf("<div class=\"flex join-point function-option fo-directive\"><span class=\"type\">Has directive</span><code>//%s</code></div>", fo.name)
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

func (fo oneOfFunctions) evaluate(info *functionInformation) bool {
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

func (fo oneOfFunctions) toHTML() string {
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

func (fo *receives) evaluate(info *functionInformation) bool {
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

func (fo *receiver) evaluate(info *functionInformation) bool {
	return info.Receiver != nil && fo.typeName.MatchesDefinition(info.Receiver, info.ImportPath)
}

func (fo *receiver) impliesImported() []string {
	return nil
}

func (fo *receiver) AsCode() jen.Code {
	return jen.Qual(pkgPath, "Receiver").Call(fo.typeName.AsCode())
}

func (fo *receiver) toHTML() string {
	return fmt.Sprintf(`<div class="flex join-point function-option fo-receiver"><span class="type">Is method of</span>%s</div>`, fo.typeName.RenderHTML())
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

func (s *funcBody) ImpliesImported() []string {
	return s.up.ImpliesImported()
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

func (s *funcBody) AsCode() jen.Code {
	return jen.Qual(pkgPath, "FunctionBody").Call(s.up.AsCode())
}

func (s *funcBody) RenderHTML() string {
	return fmt.Sprintf(`<div class="join-point function-body"><span class="type pill">Function body</span><ul><li>%s</li></ul></div>`, s.up.RenderHTML())
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
