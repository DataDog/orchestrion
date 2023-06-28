// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package main

import (
	"go/token"
	"log"
	"os"
	"reflect"
	"strings"

	"github.com/dave/dst"
	"github.com/dave/dst/decorator"
	"github.com/dave/dst/decorator/resolver/goast"
	"github.com/dave/dst/decorator/resolver/guess"
)

func ReplaceCall(pkg, function, targetPkg, targetFunc string) func(dst.Node) bool {
	return func(n dst.Node) bool {
		c := n.(*dst.CallExpr)
		if id, ok := c.Fun.(*dst.Ident); ok {
			if id.Path == pkg && id.Name == function {
				id.Path = targetPkg
				id.Name = targetFunc
				return true
			}
		}
		return false
	}
}

func WrapCall(pkg, function, targetPkg, targetFunc string) func(dst.Node) bool {
	return func(n dst.Node) bool {
		c := n.(*dst.CallExpr)
		if id, ok := c.Fun.(*dst.Ident); ok {
			if id.Path == pkg && id.Name == function {
				id.Path = targetPkg
				id.Name = targetFunc

				orig := dst.Clone(n).(*dst.CallExpr)
				c.Fun = &dst.Ident{Name: targetFunc, Path: targetPkg}
				c.Args = []dst.Expr{orig}
				return true
			}
		}
		return false
	}
}

const (
	dd_startinstrument = "//dd:startinstrument"
	dd_endinstrument   = "//dd:endinstrument"
	dd_ignore          = "//dd:ignore"
)

func hasLabel(label string, decs []string) bool {
	for _, v := range decs {
		if strings.HasPrefix(v, label) {
			return true
		}
	}
	return false
}

func shouldSkip(n dst.Node) bool {
	decos := n.Decorations().Start.All()
	return hasLabel(dd_startinstrument, decos) ||
		hasLabel(dd_ignore, decos)
}

type instrument struct {
	t    reflect.Type
	f    func(dst.Node) bool
	tags [2]string
}

type instrumentor struct {
	instruments []instrument
}

func (i *instrumentor) Visit(n dst.Node) dst.Visitor {
	if n == nil {
		return nil
	}
	t := reflect.TypeOf(n)
	if shouldSkip(n) {
		// TODO: should we return i?
		// What if there are other calls we need to instrument lower down the tree?
		return nil
	}

	if t == reflect.TypeOf(&dst.AssignStmt{}) {
		a := &nestedInstrumentor{
			instruments: i.instruments,
		}
		dst.Walk(a, n)
		if len(a.applyTags) > 0 {
			n.Decorations().Start.Append(a.applyTags[0])
			n.Decorations().End.Append("\n", a.applyTags[1])
		}
		return nil
	}

	for _, instr := range i.instruments {
		if instr.t == t {
			if instr.f(n) {
				n.Decorations().Start.Append(instr.tags[0])
				n.Decorations().End.Append("\n", instr.tags[1])
			}
		}
	}
	return i
}

// nestedInstrumentor is used to correctly place //dd:* tags around
// things like assignments, where the actual node we're instrumenting is nested
// in a node higher on the tree.
//
// For instance, in an assignment such as:
//
//	foo := pkg.Bar()
//
// when instrumenting pkg.Bar(), we don't want to apply the label to the CallExpr, since
// it results in the following:
//
//	foo := //dd:startinstrument
//		pkg.Bar()
//	//dd:endinstrument
//
// Instead, we want to apply the label to the assignment node itself.
type nestedInstrumentor struct {
	instruments []instrument
	applyTags   []string
}

func (i *nestedInstrumentor) Visit(n dst.Node) dst.Visitor {
	if n == nil {
		return nil
	}
	t := reflect.TypeOf(n)
	if shouldSkip(n) {
		// TODO: should we return i?
		// What if there are other calls we need to instrument lower down the tree?
		return nil
	}
	for _, instr := range i.instruments {
		if instr.t == t {
			if instr.f(n) {
				i.applyTags = instr.tags[:]
			}
		}
	}
	return i
}

func runInstrumentation(in, outname string) {
	file, err := os.Open(in)
	if err != nil {
		log.Fatalf("error opening file: %v", err)
	}
	defer file.Close()

	fset := token.NewFileSet()
	d := decorator.NewDecoratorWithImports(fset, in, goast.New())
	f, err := d.Parse(file)
	if err != nil {
		log.Fatalf("error parsing content in %s: %v", in, err)
	}

	inst := instrumentor{
		[]instrument{
			instrument{
				t:    reflect.TypeOf(&dst.CallExpr{}),
				f:    ReplaceCall("fmt", "Printf", "github.com/datadog/format", "Printf"),
				tags: [2]string{dd_startinstrument, dd_endinstrument},
			},
			instrument{
				t:    reflect.TypeOf(&dst.CallExpr{}),
				f:    WrapCall("fmt", "Sprintf", "github.com/datadog/format", "SprintWrap"),
				tags: [2]string{dd_startinstrument, dd_endinstrument},
			},
		},
	}

	for _, d := range f.Decls {
		dst.Walk(&inst, d)
	}

	res := decorator.NewRestorerWithImports(outname, guess.New())
	outf, err := os.Create(outname)
	if err != nil {
		log.Fatalf("error opening file: %v", err)
	}
	defer outf.Close()

	err = res.Fprint(outf, f)
	if err != nil {
		log.Fatalf("error writing file: %v", err)
	}
}

func main() {
	runInstrumentation("myfile.go.test", "myfile.go.out")
	runInstrumentation("myfile.go.out", "myfile.go.out2")
}
