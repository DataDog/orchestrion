// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

// Package injector provides a facility to inject code into go programs, either
// in source (intended to be checked in by the user) or at compilation time
// (via `-toolexec`).
package injector

import (
	"bytes"
	"context"
	"fmt"
	"go/parser"
	"go/token"
	"os"

	"github.com/datadog/orchestrion/internal/injector/typed"
	"github.com/dave/dst"
	"github.com/dave/dst/decorator"
	"github.com/dave/dst/dstutil"
)

type (
	// Injector injects go code into a program.
	Injector struct {
		fileset   *token.FileSet
		decorator *decorator.Decorator
		restorer  *decorator.Restorer
		opts      InjectorOptions
	}

	// ModifiedFileFn is called with the original file and must return the path to use when writing a modified version.
	ModifiedFileFn func(string) string

	InjectorOptions struct {
		// ModifiedFile is called to obtain the file name to use when writing a modified file. If nil, the original file is
		// overwritten in-place.
		ModifiedFile ModifiedFileFn
		// Injections is the set of configured injections to attempt.
		Injections []Injection
		// PreserveLineInfo enables emission of //line directives to preserve line information from the original file, so
		// that stack traces resolve to the original source code. This is strongly recommended when performing compile-time
		// injection.
		PreserveLineInfo bool
	}
)

// NewInjector creates a new injector with the specified options.
func NewInjector(opts InjectorOptions) *Injector {
	fileset := token.NewFileSet()
	return &Injector{
		fileset:   fileset,
		decorator: decorator.NewDecorator(fileset),
		restorer:  decorator.NewRestorer(),
		opts:      opts,
	}
}

type (
	// Result describes the result of an injection operation.
	Result struct {
		References typed.ReferenceMap // New package references injected into the file and what kind of reference they are
		Filename   string             // The file name of the output file (may be different from the input file)
		Modified   bool               // Whether the file was modified
	}
)

// Injects code in the specified file.
func (i *Injector) InjectFile(filename string) (res Result, err error) {
	res.Filename = filename

	src, err := os.ReadFile(filename)
	if err != nil {
		return res, err
	}

	file, err := i.decorator.ParseFile(filename, src, parser.ParseComments)
	if err != nil {
		return res, err
	}

	res.Modified, res.References, err = i.inject(file)
	if err != nil {
		return res, err
	}

	if res.Modified {
		buf := bytes.NewBuffer(src[:0])
		if err = i.restorer.Fprint(buf, file); err != nil {
			return res, err
		}

		res.Filename = i.outputFileFor(filename)
		err = os.WriteFile(res.Filename, postProcess(buf.Bytes()), 0o644)
	}

	return res, err
}

func (i *Injector) inject(file *dst.File) (mod bool, refs typed.ReferenceMap, err error) {
	ctx := typed.ContextWithValue(typed.ContextWithValue(context.Background(), file), &refs)

	dstutil.Apply(
		file,
		func(csor *dstutil.Cursor) bool {
			if err != nil || csor.Node() == nil {
				return false
			}

			if ddIgnored(csor.Node()) {
				return false
			}

			var changed bool
			changed, err = i.injectNode(ctx, csor)
			mod = mod || changed
			return err == nil
		},
		nil,
	)

	if mod && i.opts.PreserveLineInfo {
		i.addLineDirectives(file)
	}

	return
}

func (i *Injector) injectNode(ctx context.Context, csor *dstutil.Cursor) (mod bool, err error) {
	for _, inj := range i.opts.Injections {
		if !inj.Point.Matches(csor) {
			continue
		}
		for _, act := range inj.Actions {
			var changed bool
			changed, err = act.Apply(ctx, csor)
			if changed {
				mod = true
			}
			if err != nil {
				return
			}
		}
	}

	return
}

func (i *Injector) addLineDirectives(file *dst.File) {
	inGen := false
	var stack []bool
	dst.Inspect(file, func(node dst.Node) bool {
		if node == nil {
			if len(stack) == 0 {
				panic("popping empty stack")
			}
			inGen, stack = inGen || stack[len(stack)-1], stack[:len(stack)-1]
			return true
		}

		// Push the current node onto the stack
		defer func() { stack = append(stack, inGen) }()

		ast := i.decorator.Ast.Nodes[node]
		if ast != nil {
			if inGen {
				// We need to properly re-position this node (previous node was synthetic)
				position := i.fileset.Position(ast.Pos())
				deco := node.Decorations()
				deco.Before = dst.NewLine
				deco.Start.Append(fmt.Sprintf("//line %s:%d", position.Filename, position.Line))
				inGen = false
			}
			return true
		}

		if !inGen {
			deco := node.Decorations()
			deco.Before = dst.NewLine
			deco.Start.Prepend("//line <generated>:1")
			inGen = true
		}

		return true
	})

	if len(stack) != 0 {
		panic("finished with non-zero stack!")
	}
}

func (i *Injector) outputFileFor(filename string) string {
	if i.opts.ModifiedFile == nil {
		return filename
	}
	return i.opts.ModifiedFile(filename)
}
