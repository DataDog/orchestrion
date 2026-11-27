// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package lineinfo

import (
	"bytes"
	"go/ast"
	"go/parser"
	"go/token"
	"testing"

	"github.com/dave/dst"
	"github.com/dave/dst/decorator"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The source used by the lineinfo tests: a woven statement is inserted at the top of f's body,
// which shifts the original lines below it, exactly like advice does.
const testSource = `package p

func f() {
	probe()
}

func g() {
	probe()
}

func probe() { println("x") }
`

// wovenFile parses the test source, decorates it, and inserts a generated statement at the top of
// f's body. The statement maps to no original node, so everything below it drifts.
func wovenFile(t *testing.T) (*decorator.Decorator, *dst.File, *token.FileSet) {
	t.Helper()

	fset := token.NewFileSet()
	astFile, err := parser.ParseFile(fset, "input.go", testSource, parser.ParseComments)
	require.NoError(t, err, "the test source must parse")

	dec := decorator.NewDecorator(fset)
	file, err := dec.DecorateFile(astFile)
	require.NoError(t, err, "the test source must decorate")

	for _, decl := range file.Decls {
		fn, ok := decl.(*dst.FuncDecl)
		if !ok || fn.Name.Name != "f" || fn.Body == nil {
			continue
		}
		stmt := &dst.ExprStmt{X: &dst.CallExpr{Fun: dst.NewIdent("println"), Args: []dst.Expr{
			&dst.BasicLit{Kind: token.STRING, Value: `"woven"`},
		}}}
		fn.Body.List = append([]dst.Stmt{stmt}, fn.Body.List...)
		break
	}
	return dec, file, fset
}

// newTestAnnotator builds an annotator around a woven file whose modified bytes are written next
// to the input, like the injector does.
func newTestAnnotator(t *testing.T, dec *decorator.Decorator, file *dst.File) *annotator {
	t.Helper()
	return &annotator{
		dec:       dec,
		file:      file,
		filename:  "input.go",
		writtenAs: "input.edited.go",
		newRestorer: func(name string) *decorator.FileRestorer {
			return &decorator.FileRestorer{Restorer: decorator.NewRestorer(), Name: name}
		},
		postProcess: postProcessTest,
		anchors:     make(map[anchorKey]bool),
		genFilename: directiveFilename("input.edited.go", generatedFilename),
	}
}

// postProcessTest mirrors the injector's directive post-processing: `//line` directives only
// take effect in column one, and the printer indents decorations inside function bodies, so the
// leading white space ahead of a directive is removed before the bytes are verified or written.
func postProcessTest(src []byte) []byte {
	for i := 0; i < len(src); {
		slice := src[i:]
		trimmed := bytes.TrimLeft(slice, " \t")
		trimmedLen := len(trimmed)
		if trimmedLen != len(slice) && bytes.HasPrefix(trimmed, []byte("//line ")) {
			// This line has a line directive with leading white space: remove the white space so
			// the compiler does not ignore the directive.
			newEnd := i + trimmedLen
			copy(src[i:newEnd], trimmed)
			src = src[:newEnd]
		}

		lf := bytes.IndexByte(src[i:], '\n')
		if lf < 0 {
			break
		}
		i += lf + 1
	}
	return src
}

// formatForTest runs FormatFile on a woven fixture with the given pass limit, and returns the
// output bytes together with the file set the source was parsed in.
func formatForTest(t *testing.T, passes int) []byte {
	t.Helper()

	maxPasses = passes
	t.Cleanup(func() { maxPasses = 3 })

	dec, file, _ := wovenFile(t)
	out, err := FormatFile(Options{
		Decorator:   dec,
		File:        file,
		NewRestorer: newTestAnnotator(t, dec, file).newRestorer,
		WrittenAs:   "input.edited.go",
		PostProcess: postProcessTest,
	})
	require.NoError(t, err)
	require.NotEmpty(t, out)
	return out
}

// assertOriginalLines re-parses the output under the written path and verifies that each `probe()`
// call reports the line it has in the test source: line 4 in f, line 8 in g.
func assertOriginalLines(t *testing.T, out []byte) {
	t.Helper()
	if testing.Verbose() {
		println("===== OUTPUT =====")
		println(string(out))
		println("==================")
	}

	outFset := token.NewFileSet()
	outFile, err := parser.ParseFile(outFset, "input.edited.go", out, parser.ParseComments)
	require.NoError(t, err, "the output must parse")

	var got []int
	ast.Inspect(outFile, func(node ast.Node) bool {
		if call, ok := node.(*ast.CallExpr); ok {
			if fun, ok := call.Fun.(*ast.Ident); ok && fun.Name == "probe" {
				got = append(got, outFset.PositionFor(call.Pos(), true).Line)
			}
		}
		return true
	})
	assert.Equal(t, []int{4, 8}, got, "the original probe calls must report their source lines")
}

// TestFormatFileStableLayout checks the normal path: the loop converges within the default pass
// limit and the original code keeps its effective lines.
func TestFormatFileStableLayout(t *testing.T) {
	assertOriginalLines(t, formatForTest(t, 3))
}

// TestFormatFilePassLimit exercises the pass limit: with a single pass the loop cannot converge,
// so the dense fallback anchors every original node and the output is still correct. This also
// covers the loop exit that leads into the dense mode.
func TestFormatFilePassLimit(t *testing.T) {
	assertOriginalLines(t, formatForTest(t, 1))
}

// TestAnchorNoProgressBreak documents the condition of the no-progress break: when every reported
// need was already anchored in an earlier pass, anchor places nothing, and the loop breaks into
// the dense mode instead of spinning.
func TestAnchorNoProgressBreak(t *testing.T) {
	dec, file, _ := wovenFile(t)
	a := newTestAnnotator(t, dec, file)

	target := &dst.CallExpr{Fun: dst.NewIdent("println")}
	need := anchorNeed{
		dst:      target,
		token:    anchorPos,
		expected: token.Position{Filename: "input.go", Line: 4},
	}
	// The deduplication key carries the directive value: a line anchor's value is its expected
	// line, so the same need maps to the same key on every pass.
	a.anchors[anchorKey{node: need.dst, token: anchorPos, value: need.expected.Line}] = true

	assert.Zero(t, a.anchor([]anchorNeed{need}, false),
		"an anchor that is already in place must not be placed again")
}

// TestFallbackRender checks the measurement-free annotation: it needs no re-parse of its own
// output, it writes the base directive, and the original code keeps its effective lines even
// though the inserted statement shifts the layout.
func TestFallbackRender(t *testing.T) {
	dec, file, _ := wovenFile(t)
	a := newTestAnnotator(t, dec, file)
	a.seedBaseDirective()

	out, err := a.fallbackRender()
	require.NoError(t, err)
	require.NotEmpty(t, out)
	assert.Contains(t, string(out), "//line input.go:1:1", "the base directive must be present")
	assertOriginalLines(t, out)
}

// TestDegradedOutput checks that a verification failure never fails the build and never returns
// bytes with known mismatches: the measurement-free annotation is returned, which is the previous
// writer's behavior.
func TestDegradedOutput(t *testing.T) {
	dec, file, _ := wovenFile(t)
	a := newTestAnnotator(t, dec, file)
	a.seedBaseDirective()
	log := zerolog.Nop()

	out, err := a.degradedOutput(assert.AnError, &log)
	require.NoError(t, err)
	require.NotEmpty(t, out)
	assert.Contains(t, string(out), "//line input.go:1:1", "the fallback annotation must be used")
	assertOriginalLines(t, out)
}

// TestFormatFileVerificationFailureKeepsFallbackOutput proves end to end that a failure during the
// verify loop, after anchors were already placed, does not return the partial pass-1 bytes: the
// returned bytes come from the measurement-free annotation, so the original lines are correct.
// The failure is forced by poisoning the post-processing of the second render, which makes the
// re-parse of the printed bytes fail exactly the way an unforeseeable layout change would.
func TestFormatFileVerificationFailureKeepsFallbackOutput(t *testing.T) {
	dec, file, _ := wovenFile(t)

	renders := 0
	out, err := FormatFile(Options{
		Decorator: dec,
		File:      file,
		NewRestorer: func(name string) *decorator.FileRestorer {
			return &decorator.FileRestorer{Restorer: decorator.NewRestorer(), Name: name}
		},
		WrittenAs: "input.edited.go",
		PostProcess: func(src []byte) []byte {
			renders++
			if renders == 2 {
				// The second render's bytes must fail to re-parse.
				return []byte("this is not go source code")
			}
			return postProcessTest(src)
		},
	})
	require.NoError(t, err, "line information never fails the build")
	require.NotEmpty(t, out)
	assert.GreaterOrEqual(t, renders, 2, "the loop must have rendered at least twice")
	assert.NotContains(t, string(out), "this is not go source code", "the poisoned bytes must not be returned")
	assertOriginalLines(t, out)
}
