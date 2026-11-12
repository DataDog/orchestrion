// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package lineinfo

import (
	"bytes"
	"go/ast"
	"go/format"
	"strconv"

	"github.com/dave/dst"
	"github.com/dave/dst/decorator"
)

// fallbackRender produces the formatted bytes of the annotated tree using an annotation that
// needs no measurement: a single restoration, a walk of the resulting AST, and `//line` directives
// added wherever the measured position differs from the original one by a delta. It is the
// behavior of the writer this package had before line information was verified against the
// printed layout, and it is used as the last resort when no verified bytes exist: it cannot
// detect that the printer later changes the layout, but it always produces directives, and it
// never needs to re-parse its own output.
func (a *annotator) fallbackRender() ([]byte, error) {
	// The first restoration measures the layout the visitor compares the original positions
	// against.
	res := a.newRestorer(a.filename)
	astFile, err := res.RestoreFile(a.file)
	if err != nil {
		return nil, err
	}

	visitor := &fallbackVisitor{dec: a.dec, res: res, annotator: a}
	ast.Walk(visitor, astFile)
	if len(visitor.stack) != 0 {
		panic("non-empty stack after fallback visit is complete")
	}

	// The directives the visitor added are decorations on the dst tree, so a second restoration
	// is needed to print them, exactly like the writer did before line information was verified.
	printed := a.newRestorer(a.filename)
	printFile, err := printed.RestoreFile(a.file)
	if err != nil {
		return nil, err
	}

	buf := bytes.NewBuffer(nil)
	if err := format.Node(buf, printed.Fset, printFile); err != nil {
		return nil, err
	}
	return a.postProcess(buf.Bytes()), nil
}

// fallbackVisitor is an ast.Visitor that adds `//line` directives to the visited nodes to adjust
// their logical source location so it matches those of the original *ast.File tree.
type (
	fallbackVisitor struct {
		// dec is the decorator that transformed the original *ast.File into the visited *dst.File.
		dec *decorator.Decorator
		// res is a restorer that was used to restore the visited *dst.File into an *ast.File, and
		// hence provides source location information for the restored AST.
		res *decorator.FileRestorer
		// annotator is the owner; it provides the output file names for the directives.
		annotator *annotator

		lineInfo
		stack []dst.Node
	}
	lineInfo struct {
		curFile string // The current un-adjusted file name
		adjFile string // The current adjusted file name
		curLine int    // The current un-adjusted line number
		adjLine int    // The current adjusted line number
	}
)

var _ ast.Visitor = (*fallbackVisitor)(nil)

func (v *fallbackVisitor) Visit(node ast.Node) ast.Visitor {
	if node == nil {
		last := len(v.stack) - 1

		if node, isGenDecl := v.stack[last].(*dst.GenDecl); isGenDecl {
			// If this is a [*dst.GenDecl] with a single item that's rendered without parentheses,
			// hoist decorations from the single item to the [*dst.GenDecl] itself: it renders
			// better.
			if node.Lparen && len(node.Specs) == 1 {
				specDeco := node.Specs[0].Decorations()
				node.Decs.Start = append(node.Decs.Start, specDeco.Start...)
				specDeco.Start.Clear()
			}
		}

		// Finished visiting a node; nothing else to do.
		v.stack = v.stack[:last]
		return nil
	}

	prevInfo := v.lineInfo

	curPosition := v.res.Fset.Position(node.Pos())
	v.curFile, v.curLine = curPosition.Filename, curPosition.Line

	dstNode := v.res.Dst.Nodes[node]
	v.stack = append(v.stack, dstNode)
	if dstNode == nil {
		// Nodes such as [ast.FuncType] are not mapped directly by dst. They do not represent
		// lines that can show on stack frames.
		return v
	}

	var adjPosition string
	var adjLine int
	if orgNode := v.dec.Ast.Nodes[dstNode]; orgNode != nil {
		adjPos := v.dec.Fset.Position(orgNode.Pos())
		adjPosition, adjLine = adjPos.Filename, adjPos.Line
	}

	if adjPosition == "" {
		if _, isIdent := dstNode.(*dst.Ident); isIdent && adjLine == 0 {
			// This is a virtual [*dst.Ident] node that was created by import management. It does
			// not map back to a node in the original AST, and it's part of a [*dst.SelectorExpr]
			// that can be mapped properly, so it can be ignored here.
			return v
		}

		// This is a synthetic node...
		v.adjFile, v.adjLine = generatedFilename, 1

		if prevInfo.adjFile != generatedFilename {
			appendLineDirective(dstNode, v.directive())
		}
		return v
	}

	// Update the adjusted position to the current observed one...
	v.adjFile, v.adjLine = adjPosition, adjLine

	if curPosition.Line == adjLine && curPosition.Filename == adjPosition &&
		adjPosition == prevInfo.adjFile {
		// Current & adjusted positions match, and the adjusted file did not change: nothing to do.
		return v
	}

	if adjLine-prevInfo.adjLine == curPosition.Line-prevInfo.curLine &&
		prevInfo.curFile == curPosition.Filename &&
		prevInfo.adjFile == adjPosition {
		// Already correctly adjusted: no directive needed.
		return v
	}

	appendLineDirective(dstNode, v.directive())

	return v
}

func (v *fallbackVisitor) directive() string {
	line := strconv.Itoa(v.adjLine)
	name := v.adjFile
	if name != generatedFilename {
		// The fallback writes source positions with the same file-name rule as the verified
		// passes, so a position from an input directive keeps its relative form.
		name = v.annotator.directiveName(name)
	}
	return "//line " + name + ":" + line
}
