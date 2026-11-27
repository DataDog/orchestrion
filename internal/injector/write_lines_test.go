// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package injector

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/DataDog/orchestrion/internal/injector/aspect"
	injectparse "github.com/DataDog/orchestrion/internal/injector/parse"
	"github.com/DataDog/orchestrion/internal/yaml"
	"github.com/dave/dst/decorator"
	"github.com/dave/dst/decorator/resolver/gotypes"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/tools/go/packages"
)

// TestLineInfoAfterLayoutChanges verifies that the writer preserves the effective source position
// of retained original code in every layout shape the printer can produce, instead of relying on
// positions measured before printing. Each row drives the real injection pipeline on a complete
// source file, asserts the line-info invariant on the written bytes, and asserts its own
// precondition: that formatting the woven file without any directives really changes the probe's
// layout (or leaves it alone, for control rows), so no row can pass vacuously.
func TestLineInfoAfterLayoutChanges(t *testing.T) {
	t.Parallel()

	for _, row := range lineInfoRows {
		t.Run(row.name, func(t *testing.T) {
			t.Parallel()
			runLineInfoRow(t, row)
		})
	}
}

type lineInfoRow struct {
	name string
	// code is the complete Go source of the file to weave.
	code string
	// raw keeps the code bytes exactly as written: no trimming and no added final newline. Used
	// for CRLF and missing-EOF variants.
	raw bool
	// outDir writes the modified file to a sub-directory of the module, so the written path is
	// in a different directory from the source, like the production layout where modified files
	// go under the build directory.
	outDir bool
	// lib adds a helper sub-package to the module so that function-call join-points can wrap
	// calls to it.
	lib bool
	// aspects is the YAML document describing the aspects to apply.
	aspects string
	// probes locate nodes in the input AST and state whether the un-annotated rendering of the
	// woven file moves them relative to their anchor.
	probes []lineInfoProbe
}

type lineInfoProbe struct {
	descr string
	// node locates the probed original node in the input AST.
	node func(*ast.File) ast.Node
	// anchor locates the node the probe's printed line is compared against. It defaults to the
	// top-level declaration enclosing the probe.
	anchor func(*ast.File) ast.Node
	// changes states whether the un-annotated output is expected to move the probe relative to
	// its anchor.
	changes bool
}

// The helpers appended to most row sources. They are deliberately multi-line so that they are
// never confused with the one-line shapes under test.
const lineInfoHelpers = `
func probe() int     { return 0 }
func before() int    { return 0 }
func after() int     { return 0 }
func target(v ...int) int { return 0 }
func wide(s string) int   { return 0 }
`

const lineInfoLibSource = `package lib

func Target(v ...int) int { return 0 }

func Wrap(v int) int { return v }
`

// prependBodyAspects builds an aspect set that prepends a statement to the body of the named
// function, forcing the file through the writer without touching the shape under test.
func prependBodyAspects(funcName string) string {
	return fmt.Sprintf(`
aspects:
  - id: weave
    join-point:
      function-body:
        function:
          - name: %s
    advice:
      - prepend-statements:
          template: print("woven")
`, funcName)
}

const wrapTargetAspects = `
aspects:
  - id: wrap
    join-point:
      function-call: dummy/test/module/lib.Target
    advice:
      - wrap-expression:
          imports:
            lib: dummy/test/module/lib
          template: lib.Wrap({{ . }})
`

const addStructFieldAspects = `
aspects:
  - join-point:
      struct-definition: dummy/test/module.T
    advice:
      - add-struct-field:
          name: newField
          type: string
`

func pad(n int) string {
	return strings.Repeat("x", n)
}

var lineInfoRows = []lineInfoRow{
	{
		name: "F01/short-unmodified-one-line-body",
		code: `package p

func woven() {
	after()
}

func f() { probe() }
` + lineInfoHelpers,
		aspects: prependBodyAspects("woven"),
		probes: []lineInfoProbe{
			{descr: "call in untouched one-line body", node: callInFunc("f", "probe"), changes: false},
		},
	},
	{
		name: "F02a/comment-heavy-one-line-body",
		code: `package p

func woven() {
	after()
}

func f() int { return /*` + pad(160) + `*/ target(1) }

func following() { before() }
` + lineInfoHelpers,
		aspects: prependBodyAspects("woven"),
		probes: []lineInfoProbe{
			{descr: "comment forces the printer to expand the body", node: returnCallee("f", "target"), changes: true},
			{descr: "call after the expanded body", node: callInFunc("following", "before"), anchor: funcDeclByName("f"), changes: true},
		},
	},
	{
		name: "F02b/comment-light-one-line-body",
		code: `package p

func woven() {
	after()
}

func f() int { return /*` + pad(20) + `*/ target(1) }
` + lineInfoHelpers,
		aspects: prependBodyAspects("woven"),
		probes: []lineInfoProbe{
			{descr: "short comment does not expand the body", node: returnCallee("f", "target"), changes: false},
		},
	},
	{
		name: "F03/cgo-style-inline-line-comments",
		code: `package p

func woven() {
	after()
}

func f(x int) int { return int(( /*line :11:36*/ twice /*line :11:42*/)( /*line :11:44*/ intconv /*line :11:49*/ (x))) }

func twice(v int) int   { return v * 2 }
func intconv(v int) int  { return v }
` + lineInfoHelpers,
		aspects: prependBodyAspects("woven"),
		probes: []lineInfoProbe{
			{descr: "body with inline line directives", node: returnCallee("f", "twice"), changes: true},
		},
	},
	{
		name: "F04/woven-one-liner",
		code: `package p

import "dummy/test/module/lib"

func f() int { return before() + lib.Target() }
` + lineInfoHelpers,
		lib:     true,
		aspects: wrapTargetAspects,
		probes: []lineInfoProbe{
			{descr: "original call before the wrapped target", node: returnCallee("f", "before"), changes: false},
			{descr: "retained target inside the wrapper", node: returnCallee("f", "lib"), changes: false},
		},
	},
	{
		name: "F05/long-comment-free-one-line-body",
		code: `package p

func woven() {
	after()
}

func f() int { return wide("` + pad(160) + `") }

func following() { before() }
` + lineInfoHelpers,
		aspects: prependBodyAspects("woven"),
		probes: []lineInfoProbe{
			{descr: "width alone forces the printer to expand the body", node: returnCallee("f", "wide"), changes: true},
			{descr: "call after the expanded body", node: callInFunc("following", "before"), anchor: funcDeclByName("f"), changes: true},
		},
	},
	{
		name: "F06/multiple-statements-one-line-body",
		code: `package p

func f() { before(); target(); after() }

func woven() {
	after()
}
` + lineInfoHelpers,
		aspects: prependBodyAspects("woven"),
		probes: []lineInfoProbe{
			{descr: "first of three statements", node: callInFunc("f", "before"), changes: false},
			{descr: "last of three statements", node: lastCallInFunc("f", "after"), changes: false},
		},
	},
	{
		name: "F06b/six-statements-one-line-body",
		code: `package p

func f() { before(); probe(); target(); after(); before(); after() }

func woven() {
	after()
}
` + lineInfoHelpers,
		aspects: prependBodyAspects("woven"),
		probes: []lineInfoProbe{
			{descr: "middle of six statements", node: callInFunc("f", "probe"), changes: true},
		},
	},
	{
		name: "F07/top-level-function-literal",
		code: `package p

var f = func() { probe() }

func woven() {
	after()
}
` + lineInfoHelpers,
		aspects: prependBodyAspects("woven"),
		probes: []lineInfoProbe{
			{descr: "call in a one-line top-level literal", node: callInLit("f"), changes: false},
		},
	},
	{
		name: "F08/nested-function-literals",
		code: `package p

func f() { func() { func() { probe() }() }() }

func woven() {
	after()
}
` + lineInfoHelpers,
		aspects: prependBodyAspects("woven"),
		probes: []lineInfoProbe{
			{descr: "call in a doubly nested one-line literal", node: callInFunc("f", "probe"), changes: false},
		},
	},
	{
		name: "F09a/method-value-receiver-one-line-body",
		code: `package p

type T struct{}

func (t T) M() { probe() }

func woven() {
	after()
}
` + lineInfoHelpers,
		aspects: prependBodyAspects("woven"),
		probes: []lineInfoProbe{
			{descr: "call in a one-line value-receiver method", node: callInFunc("M", "probe"), changes: false},
		},
	},
	{
		name: "F09b/method-pointer-receiver-one-line-body",
		code: `package p

type T struct{}

func (t *T) N() { probe() }

func woven() {
	after()
}
` + lineInfoHelpers,
		aspects: prependBodyAspects("woven"),
		probes: []lineInfoProbe{
			{descr: "call in a one-line pointer-receiver method", node: callInFunc("N", "probe"), changes: false},
		},
	},
	{
		name: "F10/generic-function-one-line-body",
		code: `package p

func F[T any](x T) T { before(); return x }

func woven() {
	after()
}
` + lineInfoHelpers,
		aspects: prependBodyAspects("woven"),
		probes: []lineInfoProbe{
			{descr: "statement in a one-line generic function", node: callInFunc("F", "before"), changes: false},
		},
	},
	{
		name: "F11/following-declarations",
		code: `package p

func f() { probe() }

var x = before()

func g() { after() }

func woven() {
	after()
}
` + lineInfoHelpers,
		aspects: prependBodyAspects("woven"),
		probes: []lineInfoProbe{
			{descr: "value initializer after a one-line body", node: varCall("x", "before"), anchor: funcDeclByName("f"), changes: false},
			{descr: "function after the value initializer", node: callInFunc("g", "after"), anchor: varDeclByName("x"), changes: false},
		},
	},
	{
		name: "F12a/advice-in-first-of-three-one-liners",
		code: `package p

func a() { before() }
func b() { target() }
func c() { after() }
` + lineInfoHelpers,
		aspects: prependBodyAspects("a"),
		probes: []lineInfoProbe{
			{descr: "advised first body", node: callInFunc("a", "before"), changes: true},
			{descr: "middle body", node: callInFunc("b", "target"), anchor: funcDeclByName("a"), changes: true},
		},
	},
	{
		name: "F12b/advice-in-middle-of-three-one-liners",
		code: `package p

func a() { before() }
func b() { target() }
func c() { after() }
` + lineInfoHelpers,
		aspects: prependBodyAspects("b"),
		probes: []lineInfoProbe{
			{descr: "advised middle body", node: callInFunc("b", "target"), anchor: funcDeclByName("a"), changes: true},
			{descr: "last body", node: callInFunc("c", "after"), anchor: funcDeclByName("b"), changes: true},
		},
	},
	{
		name: "F12c/advice-in-last-of-three-one-liners",
		code: `package p

func a() { before() }
func b() { target() }
func c() { after() }
` + lineInfoHelpers,
		aspects: prependBodyAspects("c"),
		probes: []lineInfoProbe{
			{descr: "first body", node: callInFunc("a", "before"), anchor: funcDeclByName("a"), changes: false},
			{descr: "advised last body", node: callInFunc("c", "after"), anchor: funcDeclByName("b"), changes: true},
		},
	},
	{
		name: "F13/multiline-block-comment-in-call",
		code: `package p

func woven() {
	after()
}

func f() int { return wide(/* a
b */ "x") }
` + lineInfoHelpers,
		aspects: prependBodyAspects("woven"),
		probes: []lineInfoProbe{
			{descr: "body with a multiline comment in the call", node: returnCallee("f", "wide"), changes: true},
		},
	},
	{
		name: "F14/body-boundary-comments",
		code: `package p

func woven() {
	after()
}

func f() { probe() /*tail*/ } // end
` + lineInfoHelpers,
		aspects: prependBodyAspects("woven"),
		probes: []lineInfoProbe{
			{descr: "call with trailing comment in the body", node: callInFunc("f", "probe"), changes: false},
		},
	},
	{
		name: "F15a/explicit-semicolon-and-return",
		code: `package p

func woven() {
	after()
}

func f() { probe(); return }
` + lineInfoHelpers,
		aspects: prependBodyAspects("woven"),
		probes: []lineInfoProbe{
			{descr: "statement before an explicit return", node: callInFunc("f", "probe"), changes: false},
		},
	},
	{
		name: "F15b/trailing-semicolon",
		code: `package p

func woven() {
	after()
}

func f() { probe(); }
` + lineInfoHelpers,
		aspects: prependBodyAspects("woven"),
		probes: []lineInfoProbe{
			{descr: "statement before a trailing semicolon", node: callInFunc("f", "probe"), changes: false},
		},
	},
	{
		name: "F16/empty-statements-before-probe",
		code: `package p

func woven() {
	after()
}

func f() {
	;
	;
	probe()
}
` + lineInfoHelpers,
		aspects: prependBodyAspects("woven"),
		probes: []lineInfoProbe{
			{descr: "probe after empty statements", node: callInFunc("f", "probe"), changes: true},
		},
	},
	{
		name: "F17/defer-go-and-iifes-one-line",
		code: `package p

func f() { go func() { probe() }(); defer func() { before() }(); func() { target() }(); after() }

func woven() {
	after()
}
` + lineInfoHelpers,
		aspects: prependBodyAspects("woven"),
		probes: []lineInfoProbe{
			{descr: "call in a go literal body", node: callInFunc("f", "probe"), changes: false},
			{descr: "call in a defer literal body", node: callInFunc("f", "before"), changes: false},
			{descr: "call in a bare IIFE body", node: callInFunc("f", "target"), changes: false},
			{descr: "call after the immediately invoked function expressions", node: callInFunc("f", "after"), changes: false},
		},
	},
	{
		name: "F18/empty-body-with-prepend-advice",
		code: `package p

func f() {}

func g() { after() }
` + lineInfoHelpers,
		aspects: prependBodyAspects("f"),
		probes: []lineInfoProbe{
			{descr: "call in the function after the advised empty body", node: callInFunc("g", "after"), anchor: funcDeclByName("f"), changes: true},
		},
	},
	{
		name: "C01/empty-declaration-and-literal",
		code: `package p

func f() {}

var fl = func() {}

func g() { after() }
` + lineInfoHelpers,
		aspects: prependBodyAspects("g"),
		probes: []lineInfoProbe{
			{descr: "call after an empty declaration and literal", node: callInFunc("g", "after"), anchor: funcDeclByName("f"), changes: true},
		},
	},
	{
		name: "C01b/wide-header-empty-body",
		code: `package p

func f(` + pad(110) + `, v int) {}

func g() { after() }
` + lineInfoHelpers,
		aspects: prependBodyAspects("g"),
		probes: []lineInfoProbe{
			{descr: "call after a wide-header empty body", node: callInFunc("g", "after"), anchor: funcDeclByName("f"), changes: true},
		},
	},
	{
		name: "C02/comment-only-body",
		code: `package p

func woven() {
	after()
}

func f() { /*only a comment*/ }

func g() { after() }
` + lineInfoHelpers,
		aspects: prependBodyAspects("woven"),
		probes: []lineInfoProbe{
			{descr: "call after a comment-only body", node: callInFunc("g", "after"), anchor: funcDeclByName("f"), changes: false},
		},
	},
	{
		name: "C02b/comment-heavy-only-body",
		code: `package p

func woven() {
	after()
}

func f() { /*` + pad(160) + `*/ }

func g() { after() }
` + lineInfoHelpers,
		aspects: prependBodyAspects("woven"),
		probes: []lineInfoProbe{
			{descr: "call after a comment-only body wide enough to expand", node: callInFunc("g", "after"), anchor: funcDeclByName("f"), changes: true},
		},
	},
	{
		name: "C03/comment-only-multiline-body",
		code: `package p

func woven() {
	after()
}

func f() {
	// comment
}

func g() { after() }
` + lineInfoHelpers,
		aspects: prependBodyAspects("woven"),
		probes: []lineInfoProbe{
			{descr: "call after a multiline comment-only body", node: callInFunc("g", "after"), anchor: funcDeclByName("f"), changes: false},
		},
	},
	{
		name: "C04/already-multiline-body",
		code: `package p

func woven() {
	after()
}

func f() {
	before()
	target()
	after()
}
` + lineInfoHelpers,
		aspects: prependBodyAspects("woven"),
		probes: []lineInfoProbe{
			{descr: "first statement of an already multiline body", node: callInFunc("f", "before"), changes: false},
			{descr: "last statement of an already multiline body", node: lastCallInFunc("f", "after"), changes: false},
		},
	},
	{
		name: "B01/inline-if-block",
		code: `package p

var ready = true

func f() {
	if ready { probe() }
	after()
}

func woven() {
	after()
}
` + lineInfoHelpers,
		aspects: prependBodyAspects("woven"),
		probes: []lineInfoProbe{
			{descr: "call inside an inline if block", node: callInFunc("f", "probe"), changes: true},
			{descr: "call after the inline if block", node: callInFunc("f", "after"), changes: true},
		},
	},
	{
		name: "B01b/else-if-chain",
		code: `package p

var ready = true

func f() {
	if ready { before() } else if !ready { probe() } else { after() }
}

func woven() {
	after()
}
` + lineInfoHelpers,
		aspects: prependBodyAspects("woven"),
		probes: []lineInfoProbe{
			{descr: "call in the else-if branch of an inline chain", node: callInFunc("f", "probe"), changes: true},
		},
	},
	{
		name: "B01c/advice-inside-inline-block",
		code: `package p

var ready = true

func f() {
	if ready { g() }
	after()
}

func g() {
	before()
	target()
}

func g2() { probe() }
` + lineInfoHelpers,
		aspects: prependBodyAspects("g"),
		probes: []lineInfoProbe{
			{descr: "call in a function inside an inline if block", node: callInFunc("g", "target"), changes: true},
		},
	},
	{
		name: "B02a/inline-for-block",
		code: `package p

func f() {
	for i := 0; i < 1; i++ { probe() }; after()
}

func woven() {
	after()
}
` + lineInfoHelpers,
		aspects: prependBodyAspects("woven"),
		probes: []lineInfoProbe{
			{descr: "call inside an inline for block", node: callInFunc("f", "probe"), changes: true},
			{descr: "call after the inline for block", node: callInFunc("f", "after"), changes: true},
		},
	},
	{
		name: "B02b/inline-range-block",
		code: `package p

var items = []int{1, 2}

func f() {
	for range items { probe() }
	after()
}

func woven() {
	after()
}
` + lineInfoHelpers,
		aspects: prependBodyAspects("woven"),
		probes: []lineInfoProbe{
			{descr: "call inside an inline range block", node: callInFunc("f", "probe"), changes: true},
		},
	},
	{
		name: "B03a/inline-switch",
		code: `package p

var x = 1

func f() {
	switch x { case 1: probe(); default: after() }
}

func woven() {
	after()
}
` + lineInfoHelpers,
		aspects: prependBodyAspects("woven"),
		probes: []lineInfoProbe{
			{descr: "call in a case of an inline switch", node: callInFunc("f", "probe"), changes: true},
			{descr: "call in the default case", node: callInFunc("f", "after"), changes: true},
		},
	},
	{
		name: "B03b/inline-type-switch",
		code: `package p

var i any = 1

func f() {
	switch v := i.(type) { case int: probe(); _ = v; default: after() }
}

func woven() {
	after()
}
` + lineInfoHelpers,
		aspects: prependBodyAspects("woven"),
		probes: []lineInfoProbe{
			{descr: "call in a case of an inline type switch", node: callInFunc("f", "probe"), changes: true},
		},
	},
	{
		name: "B04/inline-select",
		code: `package p

var ch = make(chan int, 1)

func f() {
	select { case <-ch: probe(); default: after() }
}

func woven() {
	after()
}
` + lineInfoHelpers,
		aspects: prependBodyAspects("woven"),
		probes: []lineInfoProbe{
			{descr: "call in a case of an inline select", node: callInFunc("f", "probe"), changes: true},
		},
	},
	{
		name: "B05/labels-and-branches",
		code: `package p

var ready = true

func f() {
label:
	probe()
	if ready { goto label }
}

func woven() {
	after()
}
` + lineInfoHelpers,
		aspects: prependBodyAspects("woven"),
		probes: []lineInfoProbe{
			{descr: "labeled call", node: callInFunc("f", "probe"), changes: false},
		},
	},
	{
		name: "B06/advised-literals-defer-and-go",
		code: `package p

func f() {
	defer func() { probe() }()
	go func() { target() }()
	before()
}

func woven() {
	after()
}
` + lineInfoHelpers,
		aspects: `
aspects:
  - id: function-literals
    join-point:
      function-body:
        function:
          - name: ''
    advice:
      - prepend-statements:
          template: print("literal")
`,
		probes: []lineInfoProbe{
			{descr: "call in an advised defer literal", node: callInFunc("f", "probe"), changes: true},
			{descr: "call after the advised literals", node: callInFunc("f", "before"), changes: true},
		},
	},
	{
		name: "X01a/composite-literal-untouched",
		code: `package p

var x = []int{before(), target(), after()}

func woven() {
	after()
}
` + lineInfoHelpers,
		aspects: prependBodyAspects("woven"),
		probes: []lineInfoProbe{
			{descr: "call in a one-line composite literal", node: varCall("x", "before"), changes: false},
		},
	},
	{
		name: "X01b/composite-literal-wrapped",
		code: `package p

import "dummy/test/module/lib"

var x = []int{before(), lib.Target(), after()}
` + lineInfoHelpers,
		lib:     true,
		aspects: wrapTargetAspects,
		probes: []lineInfoProbe{
			{descr: "call before a wrapped element in a one-line literal", node: varCall("x", "before"), changes: false},
			{descr: "call after a wrapped element in a one-line literal", node: varCall("x", "after"), changes: false},
		},
	},
	{
		name: "X02/inline-struct-type-single-field",
		code: `package p

var x = struct{ A, B int }{before(), after()}

func woven() {
	after()
}
` + lineInfoHelpers,
		aspects: prependBodyAspects("woven"),
		probes: []lineInfoProbe{
			{descr: "call in a one-line struct literal", node: varCall("x", "before"), changes: false},
		},
	},
	{
		name: "X02b/inline-struct-type-expanding",
		code: `package p

var x = struct {
	A int
	B string
}{before(), "s"}

var y = struct {
	A int ` + "`json:\"a\"`" + `
}{before()}

func woven() {
	after()
}
` + lineInfoHelpers,
		aspects: prependBodyAspects("woven"),
		probes: []lineInfoProbe{
			{descr: "call in a multiline struct literal", node: varCall("x", "before"), changes: false},
			{descr: "call in a tagged-field struct literal", node: varCall("y", "before"), changes: false},
		},
	},
	{
		name: "X03/inline-interface-type",
		code: `package p

type I interface{ M(); N() }

func f() { probe() }

func woven() {
	after()
}
` + lineInfoHelpers,
		aspects: prependBodyAspects("woven"),
		probes: []lineInfoProbe{
			{descr: "call after an inline interface type", node: callInFunc("f", "probe"), anchor: typeDeclByName("I"), changes: true},
		},
	},
	{
		name: "X04/const-and-var-groups",
		code: `package p

const ( A = 1; B = 2 )

var (
	x = before()
	y = after()
)

func f() { probe() }

func woven() {
	after()
}
` + lineInfoHelpers,
		aspects: prependBodyAspects("woven"),
		probes: []lineInfoProbe{
			{descr: "call in a var group", node: varCall("x", "before"), changes: false},
			{descr: "call after a semicolon-separated const group", node: callInFunc("f", "probe"), anchor: varDeclByName("y"), changes: false},
		},
	},
	{
		name: "X05/function-literal-in-composite",
		code: `package p

var m = map[int]func(){1: func() { probe() }}

func woven() {
	after()
}
` + lineInfoHelpers,
		aspects: prependBodyAspects("woven"),
		probes: []lineInfoProbe{
			{descr: "call in a literal inside a one-line composite literal", node: callInLit("m"), changes: false},
		},
	},
	{
		name: "X06/package-initializer-wrap",
		code: `package p

import "dummy/test/module/lib"

var x = lib.Target(before(), after())

func f() { probe() }
` + lineInfoHelpers,
		lib:     true,
		aspects: wrapTargetAspects,
		probes: []lineInfoProbe{
			{descr: "argument of a wrapped package-level call", node: varCall("x", "before"), changes: false},
		},
	},
	{
		name: "X07/add-struct-field-to-one-line-struct",
		code: `package p

type T struct{ A int }

func f() { probe() }
` + lineInfoHelpers,
		aspects: addStructFieldAspects,
		probes: []lineInfoProbe{
			{descr: "field of a struct the advice expands", node: structFieldByName("T", "A"), changes: true},
		},
	},
	{
		name: "D01/existing-line-directive",
		code: `//line virtual.go:200
package p

func f() { probe() }

func woven() {
	after()
}
` + lineInfoHelpers,
		aspects: prependBodyAspects("woven"),
		probes: []lineInfoProbe{
			{descr: "call after a file-level line directive", node: callInFunc("f", "probe"), changes: false},
		},
	},
	{
		name: "D02/block-line-directive-and-empty-filename",
		code: `package p

/*line virtual.go:200:7*/ func f() { probe() }

/*line :205*/ func g() { before() }

func woven() {
	after()
}
` + lineInfoHelpers,
		aspects: prependBodyAspects("woven"),
		probes: []lineInfoProbe{
			{descr: "call after a block line directive", node: callInFunc("f", "probe"), changes: false},
			{descr: "call after an empty-filename directive", node: callInFunc("g", "before"), changes: false},
		},
	},
	{
		name:    "D08/written-in-different-directory",
		code:    "//line virtual.go:200\npackage p\n\n" + "func f() { probe() }\n\nfunc woven() {\n\tafter()\n}\n" + lineInfoHelpers,
		raw:     true,
		outDir:  true,
		aspects: prependBodyAspects("woven"),
		probes: []lineInfoProbe{
			{descr: "call after a relative input directive, written to another directory", node: callInFunc("f", "probe"), changes: false},
		},
	},
	{
		name:    "D03/build-tags",
		code:    "//go:build !nonexistenttag\n\npackage p\n\n" + "func f() { probe() }\n\nfunc woven() {\n\tafter()\n}\n" + lineInfoHelpers,
		aspects: prependBodyAspects("woven"),
		probes: []lineInfoProbe{
			{descr: "call after a build-tag header", node: callInFunc("f", "probe"), changes: false},
		},
	},
	{
		name:    "D03b/legacy-plus-build-tag",
		code:    "// +build !nonexistenttag\n\npackage p\n\n" + "func f() { probe() }\n\nfunc woven() {\n\tafter()\n}\n" + lineInfoHelpers,
		aspects: prependBodyAspects("woven"),
		probes: []lineInfoProbe{
			{descr: "call after a legacy build-tag header", node: callInFunc("f", "probe"), changes: false},
		},
	},
	{
		name: "D04a/crlf-comment-heavy-body",
		code: strings.ReplaceAll(`package p

func woven() {
	after()
}

func f() int { return /*`+pad(160)+`*/ target(1) }

func following() { before() }
`+lineInfoHelpers, "\n", "\r\n"),
		raw:     true,
		aspects: prependBodyAspects("woven"),
		probes: []lineInfoProbe{
			{descr: "CRLF source with a comment-heavy body", node: returnCallee("f", "target"), changes: true},
		},
	},
	{
		name: "D04b/crlf-woven-one-liner",
		code: strings.ReplaceAll(`package p

import "dummy/test/module/lib"

func f() int { return before() + lib.Target() }
`+lineInfoHelpers, "\n", "\r\n"),
		raw:     true,
		lib:     true,
		aspects: wrapTargetAspects,
		probes: []lineInfoProbe{
			{descr: "CRLF source with a wrapped one-liner", node: returnCallee("f", "before"), changes: false},
		},
	},
	{
		name:    "D04c/crlf-existing-line-directive",
		code:    strings.ReplaceAll("//line virtual.go:200\npackage p\n\nfunc f() { probe() }\n\nfunc woven() {\n\tafter()\n}\n"+lineInfoHelpers, "\n", "\r\n"),
		raw:     true,
		aspects: prependBodyAspects("woven"),
		probes: []lineInfoProbe{
			{descr: "CRLF source with a file-level line directive", node: callInFunc("f", "probe"), changes: false},
		},
	},
	{
		name: "D05a/missing-final-newline",
		code: `package p

func f() { probe() }

func woven() {
	after()
}
` + strings.TrimSuffix(lineInfoHelpers, "\n"),
		raw:     true,
		aspects: prependBodyAspects("woven"),
		probes: []lineInfoProbe{
			{descr: "source without a final newline", node: callInFunc("f", "probe"), changes: false},
		},
	},
	{
		name: "D05b/unicode-and-blank-lines",
		code: `package p

var ünï = 1 // ünïcödé comment

func f() { probe() }


func woven() {
	after()
}
` + lineInfoHelpers,
		aspects: prependBodyAspects("woven"),
		probes: []lineInfoProbe{
			{descr: "call between unicode declarations", node: callInFunc("f", "probe"), changes: false},
		},
	},
	{
		name: "D06/indented-line-directive-in-body",
		code: `package p

func f() {
	//line virtual.go:200
	probe()
}

func woven() {
	after()
}
` + lineInfoHelpers,
		aspects: prependBodyAspects("woven"),
		probes: []lineInfoProbe{
			{descr: "call after an indented directive the input scanner ignores", node: callInFunc("f", "probe"), changes: false},
		},
	},
	{
		name: "D07/unsorted-imports",
		code: `package p

import ( "strings"; "fmt" )

var x = strings.Contains("a", "b")
var y = fmt.Sprint(x)

func f() { probe() }

func woven() {
	after()
}
` + lineInfoHelpers,
		aspects: prependBodyAspects("woven"),
		probes: []lineInfoProbe{
			{descr: "call after a non-gofmt import block", node: callInFunc("f", "probe"), anchor: varDeclByName("y"), changes: false},
		},
	},
	{
		name: "A01/generated-islands",
		lib:  true,
		code: `package p

import "dummy/test/module/lib"

func f() {
	before()
	lib.Target()
	after()
}
` + lineInfoHelpers,
		aspects: `
aspects:
  - id: prepend
    join-point:
      function-body:
        function:
          - name: f
    advice:
      - prepend-statements:
          template: print("woven")
  - id: wrap
    join-point:
      function-call: dummy/test/module/lib.Target
    advice:
      - wrap-expression:
          imports:
            lib: dummy/test/module/lib
          template: lib.Wrap({{ . }})
`,
		probes: []lineInfoProbe{
			{descr: "original call before a generated island", node: callInFunc("f", "before"), changes: true},
			{descr: "original call after a generated island", node: callInFunc("f", "after"), changes: true},
		},
	},
}

// runLineInfoRow executes one row: it injects the row's aspects into the row's source through the
// real pipeline, gates the written bytes with the line invariant, and asserts the row's layout
// precondition against an un-annotated rendering of the same woven tree.
func runLineInfoRow(t *testing.T, row lineInfoRow) {
	t.Helper()
	ctx := t.Context()

	tmp := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(tmp, "go.mod"), []byte("module dummy/test/module\n\ngo 1.26\n"), 0o644))

	input := filepath.Join(tmp, "input.go")
	src := row.code
	if !row.raw {
		src = strings.TrimSpace(src) + "\n"
	}
	require.NoError(t, os.WriteFile(input, []byte(src), 0o644))
	lookup := lookupPackageIn(tmp)
	if row.lib {
		require.NoError(t, os.MkdirAll(filepath.Join(tmp, "lib"), 0o755))
		require.NoError(t, os.WriteFile(filepath.Join(tmp, "lib", "lib.go"), []byte(lineInfoLibSource), 0o644))
		// Build the helper package into an archive so the injector's importer can resolve it from
		// export data, the same way the toolchain would hand it to the injector.
		archive := filepath.Join(tmp, "lib.a")
		cmd := exec.Command("go", "build", "-o", archive, "./lib")
		cmd.Dir = tmp
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("failed to build the helper package: %v\n%s", err, out)
		}
		lookup = func(path string) (io.ReadCloser, error) {
			if path == "dummy/test/module/lib" {
				return os.Open(archive)
			}
			return lookupPackageIn(tmp)(path)
		}
	}

	// The row input must be valid Go: the injector silently skips a file that does not
	// type-check, which would make the row pass vacuously.
	vetCmd := exec.Command("go", "vet", "./...")
	vetCmd.Dir = tmp
	if vetOut, err := vetCmd.CombinedOutput(); err != nil {
		t.Fatalf("row %s: the input must type-check: %v\n%s", row.name, err, vetOut)
	}

	aspects := unmarshalAspects(t, row.aspects)

	// The package filter drops aspects whose implied imports are absent from the import map,
	// so the map must list every import of the source file (as the golden harness does).
	astFile, err := parser.ParseFile(token.NewFileSet(), input, src, parser.ParseComments)
	require.NoError(t, err, "row %s: the source must parse", row.name)
	importMap := make(map[string]string)
	for _, spec := range astFile.Imports {
		path, err := strconv.Unquote(spec.Path.Value)
		require.NoError(t, err, "row %s: failed to unquote import path %q", row.name, spec.Path.Value)
		importMap[path] = ""
	}

	modifiedFile := func(path string) string {
		return filepath.Join(filepath.Dir(path), filepath.Base(path)+".edited.go")
	}
	if row.outDir {
		modifiedFile = func(path string) string {
			return filepath.Join(tmp, "out", filepath.Base(path)+".edited.go")
		}
	}

	inj := Injector{
		ImportPath:   "dummy/test/module",
		ImportMap:    importMap,
		Lookup:       lookup,
		ModifiedFile: modifiedFile,
	}

	res, _, err := inj.InjectFiles(ctx, []string{input}, aspects)
	require.NoError(t, err, "injection failed")
	if len(res) == 0 {
		// InjectFiles silently skips packages that fail to type-check; surface the error so a
		// broken row setup does not hide behind an empty result.
		diagFset := token.NewFileSet()
		diagFiles, diagErr := injectparse.NewParser(diagFset, 1).ParseFiles(ctx, []string{input}, inj.packageFilterAspects(aspects))
		if diagErr == nil && len(diagFiles) > 0 {
			if _, tcErr := inj.typeCheck(ctx, diagFset, diagFiles); tcErr != nil {
				t.Logf("type-checking error hidden by InjectFiles: %v", tcErr)
			}
		}
	}
	require.Len(t, res, 1, "row %s must modify its file: the aspects must apply to the source", row.name)

	injected := res[input]
	written, err := os.ReadFile(injected.Filename)
	require.NoError(t, err)

	AssertFileLineInvariant(t, inj, FileLineCheck{
		Input:     input,
		Aspects:   aspects,
		Written:   written,
		WrittenAs: injected.Filename,
	})

	layout := measureUnannotatedLayout(t, inj, input, aspects)
	parents := astParentMap(layout.input)
	for _, probe := range row.probes {
		node := probe.node(layout.input)
		require.NotNil(t, node, "probe %q not found in the input source", probe.descr)

		anchorNode := enclosingTopLevelDecl(layout.input, parents, node)
		if probe.anchor != nil {
			anchorNode = probe.anchor(layout.input)
		}
		require.NotNil(t, anchorNode, "anchor for probe %q not found in the input source", probe.descr)

		inLine := layout.fset.PositionFor(node.Pos(), false).Line - layout.fset.PositionFor(anchorNode.Pos(), false).Line
		outLine, found := layout.lines[node]
		require.True(t, found, "probe %q must be retained in the woven output", probe.descr)
		anchorLine, found := layout.lines[anchorNode]
		require.True(t, found, "anchor for probe %q must be retained in the woven output", probe.descr)
		outOffset := outLine - anchorLine

		if probe.changes {
			assert.NotEqual(t, inLine, outOffset,
				"precondition failed for %q: the un-annotated rendering must change the probe's layout (input offset %d, printed offset %d)",
				probe.descr, inLine, outOffset)
		} else {
			assert.Equal(t, inLine, outOffset,
				"precondition failed for %q: the un-annotated rendering must preserve the probe's layout (input offset %d, printed offset %d)",
				probe.descr, inLine, outOffset)
		}
	}
}

// unannotatedLayout records where the original nodes of a file land when the woven tree is
// formatted without any line directives being added.
type unannotatedLayout struct {
	fset  *token.FileSet
	input *ast.File
	lines map[ast.Node]int
}

// measureUnannotatedLayout replays the injection without adding line directives, formats the
// woven tree, and records the physical line at which every retained original node is printed.
// This is the precondition measurement for the layout rows: it proves whether a row's shape really
// exercises a printer layout change.
func measureUnannotatedLayout(t *testing.T, inj Injector, input string, aspects []*aspect.Aspect) unannotatedLayout {
	t.Helper()
	ctx := t.Context()

	require.NoError(t, inj.validate())
	aspects = inj.packageFilterAspects(aspects)

	fset := token.NewFileSet()
	files, err := injectparse.NewParser(fset, 1).ParseFiles(ctx, []string{input}, aspects)
	require.NoError(t, err)
	require.Len(t, files, 1, "the row's aspects must apply to the source file")
	require.NotEmpty(t, files[0].Aspects, "the row's aspects must not be filtered out")

	checked, err := inj.typeCheck(ctx, fset, files)
	require.NoError(t, err)

	parsedFile := files[0]
	originals := make(map[ast.Node]bool, 512)
	ast.Inspect(parsedFile.AstFile, func(node ast.Node) bool {
		if node != nil {
			switch node.(type) {
			case *ast.Comment, *ast.CommentGroup:
				return false
			}
			originals[node] = true
		}
		return true
	})

	dec := decorator.NewDecoratorWithImports(fset, inj.ImportPath, gotypes.New(checked.Uses))
	dstFile, err := dec.DecorateFile(parsedFile.AstFile)
	require.NoError(t, err)

	_, err = inj.applyAspects(ctx, parameters{
		Decorator: dec,
		File:      dstFile,
		TypeInfo:  checked.Info,
		Importer:  checked.Importer,
		Aspects:   parsedFile.Aspects,
	})
	require.NoError(t, err)

	canonicalizeImports(ctx, dstFile)

	restorer := inj.newRestorer(input)
	restored, err := restorer.RestoreFile(dstFile)
	require.NoError(t, err)

	var buf bytes.Buffer
	require.NoError(t, format.Node(&buf, restorer.Fset, restored))

	outFset := token.NewFileSet()
	outFile, err := parser.ParseFile(outFset, input, buf.Bytes(), parser.ParseComments)
	require.NoError(t, err)

	pairs, err := alignAST(restored, outFile, "")
	require.NoError(t, err)

	lines := make(map[ast.Node]int, len(pairs))
	for _, pair := range pairs {
		dstNode := restorer.Dst.Nodes[pair.restored]
		if dstNode == nil {
			continue
		}
		if org := dec.Ast.Nodes[dstNode]; originals[org] {
			lines[org] = outFset.PositionFor(pair.printed.Pos(), false).Line
		}
	}
	return unannotatedLayout{fset: fset, input: parsedFile.AstFile, lines: lines}
}

func unmarshalAspects(t *testing.T, src string) []*aspect.Aspect {
	t.Helper()

	var cfg struct {
		Aspects []*aspect.Aspect `yaml:"aspects"`
	}
	require.NoError(t, yaml.UnmarshalContext(t.Context(), strings.NewReader(src), &cfg))
	require.NotEmpty(t, cfg.Aspects, "every row must configure aspects")
	return cfg.Aspects
}

func lookupPackageIn(dir string) func(string) (io.ReadCloser, error) {
	return func(path string) (io.ReadCloser, error) {
		pkgs, err := packages.Load(&packages.Config{Mode: packages.NeedExportFile, Dir: dir}, path)
		if err != nil {
			return nil, err
		}
		if len(pkgs) == 0 || pkgs[0].ExportFile == "" {
			return nil, fmt.Errorf("no export file found for %q", path)
		}
		return os.Open(pkgs[0].ExportFile)
	}
}

// The navigation helpers below locate nodes in the pre-advice input AST. They select by
// declaration or callee name, which stays unambiguous because every row controls its own source.

func funcDeclByName(name string) func(*ast.File) ast.Node {
	return func(f *ast.File) ast.Node {
		for _, decl := range f.Decls {
			if fn, ok := decl.(*ast.FuncDecl); ok && fn.Name.Name == name {
				return fn
			}
		}
		return nil
	}
}

func typeDeclByName(name string) func(*ast.File) ast.Node {
	return func(f *ast.File) ast.Node {
		for _, decl := range f.Decls {
			if gd, ok := decl.(*ast.GenDecl); ok {
				for _, spec := range gd.Specs {
					if ts, ok := spec.(*ast.TypeSpec); ok && ts.Name.Name == name {
						return ts
					}
				}
			}
		}
		return nil
	}
}

func varDeclByName(name string) func(*ast.File) ast.Node {
	return func(f *ast.File) ast.Node {
		for _, decl := range f.Decls {
			if gd, ok := decl.(*ast.GenDecl); ok && gd.Tok == token.VAR {
				for _, spec := range gd.Specs {
					if vs, ok := spec.(*ast.ValueSpec); ok && len(vs.Names) == 1 && vs.Names[0].Name == name {
						return vs
					}
				}
			}
		}
		return nil
	}
}

func varCall(varName string, callee string) func(*ast.File) ast.Node {
	return func(f *ast.File) ast.Node {
		spec := varDeclByName(varName)(f)
		if spec == nil {
			return nil
		}
		return findCallee([]ast.Node{spec}, callee)
	}
}

func callInFunc(funcName string, callee string) func(*ast.File) ast.Node {
	return func(f *ast.File) ast.Node {
		fn := funcDeclByName(funcName)(f)
		if fn == nil {
			return nil
		}
		return findCallee([]ast.Node{fn}, callee)
	}
}

func lastCallInFunc(funcName string, callee string) func(*ast.File) ast.Node {
	return func(f *ast.File) ast.Node {
		fn := funcDeclByName(funcName)(f)
		if fn == nil {
			return nil
		}
		var found ast.Node
		ast.Inspect(fn, func(node ast.Node) bool {
			if call, ok := node.(*ast.CallExpr); ok {
				if fun, ok := call.Fun.(*ast.Ident); ok && fun.Name == callee {
					found = call
				}
			}
			return true
		})
		return found
	}
}

func returnCallee(funcName string, callee string) func(*ast.File) ast.Node {
	return func(f *ast.File) ast.Node {
		fn := funcDeclByName(funcName)(f)
		if fn == nil {
			return nil
		}
		var found ast.Node
		ast.Inspect(fn, func(node ast.Node) bool {
			if found != nil {
				return false
			}
			if ret, ok := node.(*ast.ReturnStmt); ok {
				found = findCallee([]ast.Node{ret}, callee)
				return found == nil
			}
			return true
		})
		return found
	}
}

func callInLit(varName string) func(*ast.File) ast.Node {
	return func(f *ast.File) ast.Node {
		spec := varDeclByName(varName)(f).(*ast.ValueSpec)
		var found ast.Node
		ast.Inspect(spec, func(node ast.Node) bool {
			if call, ok := node.(*ast.CallExpr); ok {
				if fun, ok := call.Fun.(*ast.Ident); ok && fun.Name == "probe" {
					found = call
				}
			}
			return found == nil
		})
		return found
	}
}

func structFieldByName(typeName string, fieldName string) func(*ast.File) ast.Node {
	return func(f *ast.File) ast.Node {
		spec := typeDeclByName(typeName)(f)
		if spec == nil {
			return nil
		}
		st := spec.(*ast.TypeSpec).Type.(*ast.StructType)
		for _, field := range st.Fields.List {
			if len(field.Names) == 1 && field.Names[0].Name == fieldName {
				return field
			}
		}
		return nil
	}
}

// findCallee finds the first call to the named callee in the given trees, descending into
// nested statements and expressions. A callee matches the identifier of the call, or either
// side of a selector, so that `lib.Target` can be selected through "lib" or "Target".
func findCallee(roots []ast.Node, callee string) ast.Node {
	var found ast.Node
	for _, root := range roots {
		if root == nil {
			continue
		}
		ast.Inspect(root, func(node ast.Node) bool {
			if found != nil {
				return false
			}
			call, ok := node.(*ast.CallExpr)
			if !ok {
				return true
			}
			fun := call.Fun
			if paren, ok := fun.(*ast.ParenExpr); ok {
				fun = paren.X
			}
			switch fun := fun.(type) {
			case *ast.Ident:
				if fun.Name == callee {
					found = call
				}
			case *ast.SelectorExpr:
				if x, ok := fun.X.(*ast.Ident); ok && (x.Name == callee || fun.Sel.Name == callee) {
					found = call
				}
			}
			return found == nil
		})
	}
	return found
}

func enclosingTopLevelDecl(_ *ast.File, parents map[ast.Node]ast.Node, node ast.Node) ast.Node {
	for {
		parent, found := parents[node]
		if !found {
			return nil
		}
		if _, isFile := parent.(*ast.File); isFile {
			return node
		}
		node = parent
	}
}
