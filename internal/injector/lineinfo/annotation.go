// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package lineinfo

import (
	"bytes"
	"go/ast"
	"go/token"
	"strconv"

	"github.com/dave/dst"
	"github.com/dave/dst/decorator"
)

const generated = "<generated>"

type (
	// annotationVisitor is an ast.Visitor that adds `//line` directives to the visited nodes to
	// adjust their logical source location so it matches those of the original `*ast.File` tree.
	annotationVisitor struct {
		// dev is the decorator that transformed the original *ast.File into the visited *dst.File
		dec *decorator.Decorator
		// res is a restorer that was used to restore the visited *dst.File into an *ast.File, and hence
		// provides source location information for the restored AST.
		res *decorator.FileRestorer

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

var _ ast.Visitor = (*annotationVisitor)(nil)

func (v *annotationVisitor) Visit(node ast.Node) ast.Visitor {
	if node == nil {
		last := len(v.stack) - 1

		if node, isGenDecl := v.stack[last].(*dst.GenDecl); isGenDecl {
			// If this is a [*dst.GenDecl] with a single item that's rendered without parentheses, we
			// hoist decorations from the single item to the [*dst.GenDecl] itself, as it'll render
			// better.
			if node.Lparen && len(node.Specs) == 1 {
				specDeco := node.Specs[0].Decorations()
				node.Decs.Start = append(node.Decs.Start, specDeco.Start...)
				specDeco.Start.Clear()
			}
		}

		// Finished visiting a node, we don't have anything particular to do...
		v.stack = v.stack[:last]
		return nil
	}

	prevInfo := v.lineInfo

	curPosition := v.res.Fset.Position(node.Pos())
	v.curFile, v.curLine = curPosition.Filename, curPosition.Line

	dstNode := v.res.Dst.Nodes[node]
	v.stack = append(v.stack, dstNode)
	if dstNode == nil {
		// Nodes such as [ast.FuncType] are not mapped directly by dst... They anyway do not represent
		// lines that can show on stack frames, so it's not all that important...
		return v
	}

	// Emit a `//line <file>:1:1` directive at start of file to act as a base for all subsequent declarations.
	if prevInfo.adjFile == "" {
		prevInfo.adjFile, prevInfo.adjLine = curPosition.Filename, 1
		dstNode.Decorations().Start.Prepend(prevInfo.directive(true))
	}

	var adjPosition token.Position
	if orgNode := v.dec.Ast.Nodes[dstNode]; orgNode != nil {
		adjPosition = v.dec.Fset.Position(orgNode.Pos())
	}

	if adjPosition.Filename == "" {
		if _, isIdent := dstNode.(*dst.Ident); isIdent && adjPosition.Line == 0 {
			// This is a virtual [*dst.Ident] node that was created by import management. It does not map
			// back to a node in the original AST, and it's part of a [*dst.SelectorExpr] that we'll be
			// able to properly map; so we can safely ignore it now.
			return v
		}

		// This is a synthetic node...
		v.adjFile, v.adjLine = generated, 1

		if prevInfo.adjFile != generated {
			decs := dstNode.Decorations()
			decs.Start.Append(v.directive(false))
			if decs.Before == dst.None {
				decs.Before = dst.NewLine
			}
		}
		return v
	}

	// Update the adjusted position to the current observed one...
	v.adjFile, v.adjLine = adjPosition.Filename, adjPosition.Line

	if curPosition.Line == adjPosition.Line && curPosition.Filename == adjPosition.Filename &&
		adjPosition.Filename == prevInfo.adjFile {
		// Current & adjusted positions match, and we've not changed adjusted files -- nothing to do!
		return v
	}

	if adjPosition.Line-prevInfo.adjLine == curPosition.Line-prevInfo.curLine &&
		prevInfo.curFile == curPosition.Filename &&
		prevInfo.adjFile == adjPosition.Filename {
		// We're already correctly adjusted, so we don't need to add another directive...
		return v
	}

	decs := dstNode.Decorations()
	decs.Start.Append(v.directive(false))
	if decs.Before == dst.None {
		decs.Before = dst.NewLine
	}

	return v
}

func (l *lineInfo) directive(fileStart bool) string {
	line := strconv.FormatInt(int64(l.adjLine), 10)

	const prefix = "//line "
	builder := bytes.NewBuffer(make([]byte, 0, len(prefix)+len(l.adjFile)+1+len(line)+2))

	_, _ = builder.WriteString(prefix)
	_, _ = builder.WriteString(l.adjFile)
	switch l.adjLine {
	case 0:
		// We don't emit 0 line numbers
	case 1:
		if fileStart {
			// We emit :1:1 for line 1, so we match the output of `go tool cover` for this particular case.
			_, _ = builder.WriteString(":1:1")
			break
		}
		fallthrough
	default:
		// Otherwise, we only emit line number (no column).
		_ = builder.WriteByte(':')
		_, _ = builder.WriteString(line)
	}

	return builder.String()
}
