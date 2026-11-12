// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package lineinfo

import (
	"bytes"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"path/filepath"
	"reflect"
	"slices"
	"strconv"
	"strings"

	"github.com/dave/dst"
	"github.com/dave/dst/decorator"
	"github.com/rs/zerolog"
)

const (
	// generatedFilename is the pseudo-file name the injector attributes to nodes it created.
	generatedFilename = "<generated>"
)

// maxPasses bounds the annotate-then-measure loop. Each pass observes the layout that is actually
// printed, so a small number of passes is enough for the printed layout to become stable. It is a
// variable so that tests can exercise the pass limit through the export_test.go seam.
var maxPasses = 3

// Options carries the inputs of [FormatFile].
type Options struct {
	// Decorator is the decorator that produced the File.
	Decorator *decorator.Decorator
	// File is the *dst.File to annotate and format.
	File *dst.File
	// NewRestorer creates a new *decorator.FileRestorer for the given filename.
	NewRestorer func(string) *decorator.FileRestorer
	// WrittenAs is the path the formatted bytes will be written to. The output is verified under
	// this path, like the compiler reads it, so unanchored code is detected instead of passing
	// under the source file's name.
	WrittenAs string
	// PostProcess is applied to the formatted bytes before they are verified and written. The
	// injector uses it to move `//line` directives to column one.
	PostProcess func([]byte) []byte
	// Log is the logger to use. It may be nil.
	Log *zerolog.Logger
}

// FormatFile produces the final formatted bytes of a modified [dst.File], with `//line` directives
// computed from the layout that is actually printed rather than from a pre-print estimate.
//
// Each pass annotates the tree, restores and formats it, re-parses the printed bytes under
// writtenAs, the path the file will be written to, and checks the effective position of every
// retained original node and of the tokens the compiler records for line information. Anchors are
// added wherever the effective position drifted, and the loop repeats until the printed layout is
// stable. If the layout does not stabilize, every original node is anchored instead.
//
// The file always starts with a `//line <file>:1:1` base directive, so code before the first anchor
// is attributed to the source file rather than to the path the modified file is written to.
//
// Line information never fails a build. On a verification failure no partial output is returned:
// the output of a measurement-free annotation is returned instead, which is the previous writer's
// behavior. When even the dense fallback leaves mismatches, that output is returned and the
// residual mismatches are logged at debug level: every original node was already anchored, so no
// better option exists. Only a tree that cannot be restored or formatted at all produces an
// error, and the previous writer failed on that too.
func FormatFile(opts Options) ([]byte, error) {
	log := opts.Log
	if log == nil {
		log = &disabledLogger
	}

	a := annotator{
		dec:         opts.Decorator,
		file:        opts.File,
		filename:    opts.Decorator.Filenames[opts.File],
		writtenAs:   opts.WrittenAs,
		newRestorer: opts.NewRestorer,
		postProcess: opts.PostProcess,
		anchors:     make(map[anchorKey]bool),
		genFilename: directiveFilename(opts.WrittenAs, generatedFilename),
	}

	// Pre-process the AST to bring it closer to canonical go format. This does not affect the
	// correctness of the loop, which measures the printed layout, but it reduces the number of
	// directives needed to align the measured and printed layouts.
	canonicalizer := canonicalizationVisitor{}
	dst.Walk(&canonicalizer, opts.File)

	// The base directive anchors the start of the file to the source file itself, the way
	// `go tool cover` and the previous writer do. Without it, code before the first anchor is
	// attributed to the path the modified file is written to.
	a.seedBaseDirective()

	for range maxPasses {
		rendered, err := a.render()
		if err != nil {
			return a.degradedOutput(err, log)
		}
		needs, err := a.inspect(rendered, false)
		if err != nil {
			return a.degradedOutput(err, log)
		}
		if len(needs) == 0 {
			return rendered.data, nil
		}

		if a.anchor(needs, false) == 0 {
			// All the required anchors were already in place and the layout is still not stable:
			// fall back to the dense mode below instead of looping.
			break
		}
	}

	// Dense fallback: anchor every original node and token, then check the result once more.
	rendered, err := a.render()
	if err != nil {
		return a.degradedOutput(err, log)
	}
	denseNeeds, err := a.inspect(rendered, true)
	if err != nil {
		return a.degradedOutput(err, log)
	}
	a.anchor(denseNeeds, true)

	rendered, err = a.render()
	if err != nil {
		return a.degradedOutput(err, log)
	}
	finalNeeds, err := a.inspect(rendered, false)
	if err != nil {
		return a.degradedOutput(err, log)
	}
	if len(finalNeeds) > 0 {
		log.Debug().Str("path", a.filename).Int("mismatches", len(finalNeeds)).
			Msg("line information could not be fully stabilized, keeping the output as is")
	}
	return rendered.data, nil
}

var disabledLogger = zerolog.Nop()

// anchorToken identifies which position of a node an anchor fixes.
type anchorToken int

const (
	// anchorPos fixes the effective line of the node's first token with a `//line` directive on
	// the line above it.
	anchorPos anchorToken = iota
	// anchorLparen fixes the effective line of a call's opening parenthesis with an inline
	// `/*line*/` directive attached between the call's function expression and the parenthesis.
	anchorLparen
	// anchorTokPos fixes the effective line of an assignment's operator with an inline directive
	// attached to the end of the last left-hand side operand.
	anchorTokPos
	// anchorRbrace fixes the effective line of a function body's closing brace, the implicit
	// return line, with an inline directive attached to the end of the body's last statement.
	anchorRbrace
	// anchorGenerated attributes a generated node to the generated pseudo-file.
	anchorGenerated
)

// anchorKey deduplicates anchors across passes. An inline directive's value depends on the
// printed layout, so a changed value is a new anchor: the corrected directive is placed next to
// the stale one, and the token takes the position of the last directive before it.
type anchorKey struct {
	node  dst.Node
	token anchorToken
	value int
}

// anchorNeed describes one position and the effective position it must get. printedLine is the
// physical line the position was printed on, used to avoid stacking redundant directives: the
// first anchor of a printed line fixes that line for everything that shares it. carrierEndLine is
// the physical line the inline directive's carrier node ends on: an inline directive applies to
// the next token, so its value must be adjusted by the physical distance between the carrier and
// the token.
type anchorNeed struct {
	dst            dst.Node
	token          anchorToken
	expected       token.Position
	printedLine    int
	carrierEndLine int
}

type annotator struct {
	dec         *decorator.Decorator
	file        *dst.File
	filename    string
	writtenAs   string
	newRestorer func(string) *decorator.FileRestorer
	postProcess func([]byte) []byte
	anchors     map[anchorKey]bool

	// genFilename is the file name the compiler resolves a relative `<generated>` directive to.
	genFilename string

	// outFset is the file set of the printed output of the pass being inspected.
	outFset *token.FileSet
}

// rendered holds one measure of the annotated tree: the bytes that would be written, and the
// restored AST they were printed from.
type rendered struct {
	data     []byte
	restorer *decorator.FileRestorer
	restored *ast.File
}

func (a *annotator) render() (rendered, error) {
	restorer := a.newRestorer(a.filename)
	astFile, err := restorer.RestoreFile(a.file)
	if err != nil {
		return rendered{}, err
	}
	buf := bytes.NewBuffer(nil)
	if err := format.Node(buf, restorer.Fset, astFile); err != nil {
		return rendered{}, err
	}
	return rendered{data: a.postProcess(buf.Bytes()), restorer: restorer, restored: astFile}, nil
}

// inspect re-parses the printed bytes under the written path and reports the positions that need
// an anchor. In mismatch mode (all=false) only drifted positions are reported. In dense mode
// (all=true) every original node, owned token, and attributable generated node is reported, so
// the caller can anchor all of them.
func (a *annotator) inspect(r rendered, all bool) ([]anchorNeed, error) {
	outFset := token.NewFileSet()
	outFile, err := parser.ParseFile(outFset, a.writtenAs, r.data, parser.ParseComments)
	if err != nil {
		return nil, err
	}

	rel := treeRelations{
		parents:    make(map[ast.Node]ast.Node, 1024),
		delegating: make(map[ast.Node]ast.Node, 256),
	}
	pairs, err := align(r.restored, outFile, &rel)
	if err != nil {
		return nil, err
	}
	a.outFset = outFset
	parents := rel.parents

	// The restored tree's parents are only needed to expand the collapsed selectors the restorer
	// synthesizes; build them on first use so sources without qualified identifiers skip the walk.
	var restoredParents map[ast.Node]ast.Node
	restoredParentsOf := func() map[ast.Node]ast.Node {
		if restoredParents == nil {
			restoredParents = astParents(r.restored)
		}
		return restoredParents
	}

	printedToDst := make(map[ast.Node]dst.Node, 64)
	for _, pair := range pairs {
		if node := r.restorer.Dst.Nodes[pair.restored]; node != nil {
			printedToDst[pair.printed] = node
		}
	}

	// The original classification of every printed node, inherited from the nearest classified
	// ancestor for nodes the restorer synthesized without a dst identity (such as function types).
	printedOriginal := make(map[ast.Node]bool, len(pairs))
	original := make([]bool, len(pairs))
	delegatingChild := func(n ast.Node) ast.Node { return rel.delegating[n] }
	for i, pair := range pairs {
		dstNode := r.restorer.Dst.Nodes[pair.restored]
		if dstNode == nil {
			original[i] = false
			continue
		}
		org := a.dec.Ast.Nodes[dstNode]
		if org == nil {
			// The restorer synthesizes dst nodes to expand a path-qualified identifier that the
			// decorator collapsed (e.g. the operands of `errors.New`): the expansion components
			// belong to the original selector they were expanded from.
			org = selectorExpansionOf(pair.restored, restoredParentsOf(), r.restorer, a.dec)
		}
		original[i] = isOriginal(a.dec, org, a.filename)
		printedOriginal[pair.printed] = original[i]
	}
	var resolveOriginal func(node ast.Node) bool
	resolveOriginal = func(node ast.Node) bool {
		if node == nil {
			return true
		}
		if classified, ok := printedOriginal[node]; ok {
			return classified
		}
		return resolveOriginal(parents[node])
	}

	var needs []anchorNeed
	for i, pair := range pairs {
		dstNode := r.restorer.Dst.Nodes[pair.restored]
		if dstNode == nil {
			// Nodes synthesized by the restorer that carry no source identity.
			continue
		}

		if !original[i] {
			need := a.generatedAnchorNeed(genContext{
				pair:            pair,
				dstNode:         dstNode,
				parents:         parents,
				printedToDst:    printedToDst,
				all:             all,
				resolveOriginal: resolveOriginal,
				delegatingChild: delegatingChild,
			})
			if need != nil {
				needs = append(needs, *need)
			}
			continue
		}

		org := a.dec.Ast.Nodes[dstNode]
		if org == nil {
			// An expansion component of a collapsed selector: its identity is the selector.
			org = selectorExpansionOf(pair.restored, restoredParentsOf(), r.restorer, a.dec)
		}

		// The node's own position.
		if block, isBlock := pair.printed.(*ast.BlockStmt); !isBlock || !isBodyBlock(block, parents) {
			// The opening brace of a function body cannot be re-anchored: a directive line before
			// it would insert a semicolon after the parameter list and break the code. The brace
			// carries no line information; its declaration is anchored, and its closing brace is
			// covered by the token gate below.
			want := a.dec.Fset.PositionFor(org.Pos(), true)
			got := outFset.PositionFor(pair.printed.Pos(), true)
			if !firstTokenGenerated(pair.printed, resolveOriginal, delegatingChild) &&
				(all || want.Filename != got.Filename || want.Line != got.Line) {
				needs = append(needs, anchorNeed{
					dst:         dstNode,
					token:       anchorPos,
					expected:    want,
					printedLine: outFset.PositionFor(pair.printed.Pos(), false).Line,
				})
			}
		}

		// The tokens the compiler records for line information.
		if call, ok := org.(*ast.CallExpr); ok {
			if out, ok := pair.printed.(*ast.CallExpr); ok {
				needs = append(needs, a.tokenNeed(dstNode, all, tokenPositions{
					want:    call.Lparen,
					printed: out.Lparen,
					carrier: out.Fun.End(),
				}, anchorLparen)...)
			}
		}
		if assign, ok := org.(*ast.AssignStmt); ok {
			if out, ok := pair.printed.(*ast.AssignStmt); ok {
				lhsEnd := out.Pos()
				if len(out.Lhs) > 0 {
					lhsEnd = out.Lhs[len(out.Lhs)-1].End()
				}
				needs = append(needs, a.tokenNeed(dstNode, all, tokenPositions{
					want:    assign.TokPos,
					printed: out.TokPos,
					carrier: lhsEnd,
				}, anchorTokPos)...)
			}
		}
		if brace, ok := bodyRbrace(org, pair.printed); ok {
			needs = append(needs, a.tokenNeed(dstNode, all, brace, anchorRbrace)...)
		}
	}

	// A stable sort keeps the tree walk order for needs on the same printed line: a node's own
	// position need always precedes the needs of its descendants, so the first anchor placed on a
	// line is the one that covers the whole line instead of an inline anchor that would split it.
	slices.SortStableFunc(needs, func(x, y anchorNeed) int {
		return x.printedLine - y.printedLine
	})
	return needs, nil
}

// generatedAnchorNeed reports the anchor need for a generated node, or nil when the node is
// already attributed or not attributable. Generated code is attributed to the generated
// pseudo-file at every node kind, as the previous writer did, so generated expressions inside
// original statements map to [generatedFilename] too. Signatures are the exception: the previous
// writer left generated signature identifiers on the line they are printed on, they carry no line
// information of their own, and no directive can be placed inside a signature without splitting
// it. Function literal and declaration bodies are anchored through their enclosing function,
// because a directive line before a body's opening brace would insert a semicolon.
func (a *annotator) generatedAnchorNeed(ctx genContext) *anchorNeed {
	pair, dstNode, parents := ctx.pair, ctx.dstNode, ctx.parents
	if inSignature(pair.printed, parents) {
		return nil
	}
	if field, isField := pair.printed.(*ast.Field); isField && !isStructField(field, parents) {
		// A signature field carries no line information of its own and follows the printed line.
		return nil
	}
	if startsAtOriginalToken(pair.printed, ctx.resolveOriginal, ctx.delegatingChild) {
		// The node wraps an original operand at its first token (e.g. the call generated around
		// `pkgReq` in wrap advice): the operand carries the line, and the wrapper legitimately
		// follows it, exactly as the previous writer let it.
		return nil
	}
	got := a.outFset.PositionFor(pair.printed.Pos(), true)
	if !ctx.all && got.Filename == a.genFilename {
		return nil
	}

	target := dstNode
	if block, isBlock := pair.printed.(*ast.BlockStmt); isBlock && isBodyBlock(block, parents) {
		// Anchor through the enclosing function or control statement: a directive line before the
		// block's own opening brace would insert a semicolon after the header.
		if fn := parents[block]; fn != nil {
			if d, ok := ctx.printedToDst[fn]; ok && d != nil {
				target = d
			}
		}
	}
	return &anchorNeed{
		dst:         target,
		token:       anchorGenerated,
		expected:    token.Position{Filename: a.genFilename, Line: 1},
		printedLine: a.outFset.PositionFor(pair.printed.Pos(), false).Line,
	}
}

// genContext carries the state the generated-node gate needs: the aligned pair, its dst node,
// the printed tree's parents and dst mappings, whether dense mode is on, and the original
// classification resolver.
type genContext struct {
	pair            alignedPair
	dstNode         dst.Node
	parents         map[ast.Node]ast.Node
	printedToDst    map[ast.Node]dst.Node
	all             bool
	resolveOriginal func(ast.Node) bool
	delegatingChild func(ast.Node) ast.Node
}

// tokenPositions groups the positions of an owned token: its position in the original source, its
// position in the printed output, and the end of the node the inline directive attaches to.
type tokenPositions struct {
	want    token.Pos
	printed token.Pos
	carrier token.Pos
}

// tokenNeed reports the anchor need, if any, for an owned token of an original node. The token is
// anchored with an inline directive when its effective line drifted, or in dense mode.
func (a *annotator) tokenNeed(dstNode dst.Node, all bool, pos tokenPositions, kind anchorToken) []anchorNeed {
	want := a.dec.Fset.PositionFor(pos.want, true)
	got := a.outFset.PositionFor(pos.printed, true)
	if !all && want.Filename == got.Filename && want.Line == got.Line {
		return nil
	}
	return []anchorNeed{{
		dst:            dstNode,
		token:          kind,
		expected:       want,
		printedLine:    a.outFset.PositionFor(pos.printed, false).Line,
		carrierEndLine: a.outFset.PositionFor(pos.carrier, false).Line,
	}}
}

// anchor places the directives required by the reported needs. The first anchor of a printed line
// fixes that line for every node that shares it. Beyond that, an anchor is skipped when the delta
// of the previously placed anchor already gives the expected line: the verification loop and the
// dense fallback protect the correctness of skipping. In dense mode every reported need is
// anchored, because the loop already failed to stabilize the layout. It returns the number of new
// anchors placed.
func (a *annotator) anchor(needs []anchorNeed, dense bool) int {
	placed := 0
	perLine := make(map[int]token.Position, len(needs))
	// The running state of the delta skip: the printed line the last placed anchor established,
	// in next-pass coordinates, and the effective position it set there.
	stateLine, stateEff := -1, token.Position{}
	lineShifts := 0

	for _, need := range needs {
		printed := need.printedLine + lineShifts

		if stateLine >= 0 && need.expected.Filename == stateEff.Filename &&
			(need.token == anchorGenerated || (!dense && need.expected.Line == stateEff.Line+(printed-stateLine))) {
			// The previous anchor's delta already gives this position. A generated anchor only
			// needs the attribution to be the generated pseudo-file: the line number inside a
			// generated region is not gated, so one directive per contiguous region is enough.
			continue
		}
		if fixed, ok := perLine[need.printedLine]; ok && fixed.Filename == need.expected.Filename && fixed.Line == need.expected.Line {
			// This printed line is already anchored to this position. The printed line number is
			// used without the shift: anchors placed in this call move whole printed lines, so
			// nodes that shared a line before still share one after the shift.
			continue
		}

		// An inline directive applies to the next token, so its value must be the wanted line
		// minus the physical distance between the directive, at the end of its carrier node, and
		// the token it anchors, which the directive itself cannot cross. A line directive sets the
		// line of the node directly below it, so its value is the wanted line itself.
		value := need.expected.Line
		if need.token == anchorLparen || need.token == anchorTokPos || need.token == anchorRbrace {
			value = need.expected.Line - (need.printedLine - need.carrierEndLine)
			if value < 1 {
				// The directive cannot express this position: leave it for the next pass.
				continue
			}
		}

		key := anchorKey{node: need.dst, token: need.token, value: value}
		if a.anchors[key] {
			// The anchor with this exact value was placed in an earlier pass. It still defines
			// the effective position of this printed line, so record it for the needs that
			// share the line and for the delta of the needs that follow.
			perLine[need.printedLine] = need.expected
			if need.token == anchorPos || need.token == anchorGenerated {
				stateLine, stateEff = printed, need.expected
			} else {
				stateLine, stateEff = printed, token.Position{Filename: need.expected.Filename, Line: value}
			}
			continue
		}

		switch need.token {
		case anchorPos:
			appendLineDirective(need.dst, a.directiveLine(need.expected))
			perLine[need.printedLine] = need.expected
			stateLine, stateEff = printed+1, need.expected
			lineShifts++
		case anchorGenerated:
			appendLineDirective(need.dst, "//line "+generatedFilename+":1")
			perLine[need.printedLine] = need.expected
			stateLine, stateEff = printed+1, need.expected
			lineShifts++
		case anchorLparen, anchorTokPos, anchorRbrace:
			pos := need.expected
			pos.Line = value
			a.placeInlineAnchor(need, pos)
			perLine[need.printedLine] = need.expected
			stateLine, stateEff = printed, pos
		}

		a.anchors[key] = true
		placed++
	}
	return placed
}

// appendLineDirective appends a `//line` directive to a node's start decorations. `//line`
// directives only take effect in column one, and postProcess moves them there. A directive
// appended behind existing start decorations (for example an input line directive) would share
// its line with them and stay inert, so it is moved to its own line first.
func appendLineDirective(node dst.Node, directive string) {
	decs := node.Decorations()
	if len(decs.Start.All()) > 0 {
		decs.Start.Append("\n")
	}
	decs.Start.Append(directive)
	if decs.Before == dst.None {
		decs.Before = dst.NewLine
	}
}

// seedBaseDirective prepends the file-start base directive to the file. It is emitted even when
// nothing else drifted, so code without an anchor maps to the source file, not to the path the
// modified file is written to.
func (a *annotator) seedBaseDirective() {
	if a.anchors[anchorKey{node: a.file, token: anchorPos, value: 1}] {
		return
	}
	a.file.Decorations().Start.Prepend("//line " + a.filename + ":1:1")
	a.anchors[anchorKey{node: a.file, token: anchorPos, value: 1}] = true
}

// placeInlineAnchor attaches an inline directive for an owned token. The directive lands on the
// line its carrier node ends on and applies to the next token.
func (a *annotator) placeInlineAnchor(need anchorNeed, pos token.Position) {
	switch need.token {
	case anchorLparen:
		if call, ok := need.dst.(*dst.CallExpr); ok && call.Fun != nil {
			// The dedicated Fun decoration slot renders between the function expression and
			// the parenthesis, which is exactly where the directive must be.
			call.Decs.Fun.Append(inlineDirective(a.directiveName(pos.Filename), pos.Line))
		}
	case anchorTokPos:
		if assign, ok := need.dst.(*dst.AssignStmt); ok && len(assign.Lhs) > 0 {
			assign.Lhs[len(assign.Lhs)-1].Decorations().End.Append(
				inlineDirective(a.directiveName(pos.Filename), pos.Line))
		}
	case anchorRbrace:
		if body := bodyOf(need.dst); body != nil {
			if len(body.List) > 0 {
				body.List[len(body.List)-1].Decorations().End.Append(
					inlineDirective(a.directiveName(pos.Filename), pos.Line))
			} else {
				// An empty or comment-only body has no statement to attach to. The Lbrace
				// decoration slot renders between the opening brace and the closing one. The
				// directive is prepended so that it lands on the opening brace's line, before
				// any body comment, matching the value computed for that line.
				body.Decs.Lbrace.Prepend(inlineDirective(a.directiveName(pos.Filename), pos.Line))
			}
		}
	}
}

// directiveLine renders a `//line file:line` directive. The directive is placed on the line above
// the node it anchors, and sets the position of that node's first token.
func (a *annotator) directiveLine(pos token.Position) string {
	return "//line " + a.directiveName(pos.Filename) + ":" + strconv.Itoa(pos.Line)
}

// inlineDirective renders a `/*line file:line*/` directive. An inline directive applies to the
// immediately following token, which is how a token in the middle of a printed line is anchored.
func inlineDirective(name string, line int) string {
	return "/*line " + name + ":" + strconv.Itoa(line) + "*/"
}

// directiveName returns the file name to write in a directive. The source file's own name is
// written verbatim, absolute in production: cmd/compile keeps directive file names as they are,
// so a relative form would make runtime.Caller, panics, and pprof report a name the user does
// not recognize. A name that comes from an input directive (for example `virtual.go`) resolves to
// the same absolute name the previous writer wrote: when the modified file is written to a
// different directory, like the build directory in production, the relative form is impossible
// and the absolute name is written. The relative form is only kept when the written file is in
// the same directory as the source, which is the golden-test layout; it keeps the snapshots
// machine-independent and resolves to the same file. A preserved relative input directive then
// resolves against the written file's directory, so the following original code is re-anchored
// with the correct name; the D08 layout row covers this.
func (a *annotator) directiveName(want string) string {
	if want == a.filename {
		return want
	}
	if rel, err := filepath.Rel(filepath.Dir(a.writtenAs), want); err == nil && !strings.HasPrefix(rel, "..") {
		return rel
	}
	return want
}

// degradedOutput handles a verification failure: no pass of the loop verified cleanly, so the
// bytes of a measurement-free annotation are returned, which is the previous writer's behavior.
// Earlier passes may have produced bytes that parsed but had known mismatches; those are never
// returned, because the measurement-free annotation carries no known mismatch. Line information
// never fails a build: only a tree that cannot be restored or formatted at all produces an
// error, and the previous writer failed on that too.
func (a *annotator) degradedOutput(cause error, log *zerolog.Logger) ([]byte, error) {
	out, err := a.fallbackRender()
	if err != nil {
		return nil, err
	}
	log.Debug().Err(cause).Str("path", a.filename).
		Msg("line information verification failed, keeping a measurement-free annotation")
	return out, nil
}

// isOriginal reports whether a dst node maps back to a node of the source file being written.
// Nodes of advice templates map to a different token file, and cloned or synthesized nodes do not
// map at all: none of them are original.
func isOriginal(dec *decorator.Decorator, org ast.Node, filename string) bool {
	if org == nil {
		return false
	}
	return dec.Fset.PositionFor(org.Pos(), false).Filename == filename
}

// selectorExpansionOf returns the original AST node behind the collapsed selector a node was
// expanded from, or nil when the node is not such an expansion component. The decorator collapses
// path-qualified identifiers such as `errors.New` into a single dst identifier; when the restorer
// expands it back, the operand identifiers map to synthesized dst nodes that carry no identity of
// their own. Their identity is the original selector, and their attribution must follow it.
func selectorExpansionOf(restored ast.Node, parents map[ast.Node]ast.Node, restorer *decorator.FileRestorer, dec *decorator.Decorator) ast.Node {
	if _, ok := restored.(*ast.Ident); !ok {
		return nil
	}
	selector, ok := parents[restored].(*ast.SelectorExpr)
	if !ok {
		return nil
	}
	dstNode := restorer.Dst.Nodes[selector]
	if dstNode == nil {
		return nil
	}
	if _, ok := dstNode.(*dst.Ident); !ok {
		return nil
	}
	return dec.Ast.Nodes[dstNode]
}

// inSignature reports whether a node is part of a function signature: the parameters or results
// subtree of a function type.
func inSignature(node ast.Node, parents map[ast.Node]ast.Node) bool {
	for n := node; n != nil; {
		switch parents[n].(type) {
		case *ast.FuncType:
			return true
		case *ast.FuncDecl, *ast.FuncLit, *ast.BlockStmt:
			return false
		}
		n = parents[n]
	}
	return false
}

// isBodyBlock reports whether a block statement is the body of a function or of a control
// statement (if, for, range, switch, type switch, select). The opening brace of such a block
// follows the header's closing parenthesis or the `else` keyword, so a directive line before it
// would insert a semicolon and break the code; the brace carries no line information anyway.
func isBodyBlock(block *ast.BlockStmt, parents map[ast.Node]ast.Node) bool {
	switch parents[block].(type) {
	case *ast.FuncDecl, *ast.FuncLit, *ast.IfStmt, *ast.ForStmt, *ast.RangeStmt,
		*ast.SwitchStmt, *ast.TypeSwitchStmt, *ast.SelectStmt:
		return true
	}
	return false
}

// isStructField reports whether a field belongs to a struct type rather than a signature.
func isStructField(field *ast.Field, parents map[ast.Node]ast.Node) bool {
	list, ok := parents[field].(*ast.FieldList)
	if !ok {
		return false
	}
	_, ok = parents[list].(*ast.StructType)
	return ok
}

// bodyRbrace returns the positions of a function body's closing brace: its position in the
// original source, its position in the printed output, and the end of the body's last printed
// statement, which the inline anchor attaches to.
func bodyRbrace(org ast.Node, printed ast.Node) (brace tokenPositions, ok bool) {
	switch org := org.(type) {
	case *ast.FuncDecl:
		if out, ok := printed.(*ast.FuncDecl); ok && org.Body != nil && out.Body != nil {
			return tokenPositions{want: org.Body.Rbrace, printed: out.Body.Rbrace, carrier: lastStmtEnd(out.Body)}, true
		}
	case *ast.FuncLit:
		if out, ok := printed.(*ast.FuncLit); ok && org.Body != nil && out.Body != nil {
			return tokenPositions{want: org.Body.Rbrace, printed: out.Body.Rbrace, carrier: lastStmtEnd(out.Body)}, true
		}
	}
	return tokenPositions{}, false
}

func lastStmtEnd(body *ast.BlockStmt) token.Pos {
	if len(body.List) == 0 {
		return body.Lbrace
	}
	return body.List[len(body.List)-1].End()
}

func bodyOf(node dst.Node) *dst.BlockStmt {
	switch node := node.(type) {
	case *dst.FuncDecl:
		return node.Body
	case *dst.FuncLit:
		return node.Body
	}
	return nil
}

// directiveFilename returns the file name the compiler resolves a `//line` directive to: relative
// directive file names are joined with the directory of the file they appear in.
func directiveFilename(path string, name string) string {
	dir, _ := filepath.Split(path)
	return filepath.Join(dir, name)
}

// treeRelations carries the printed tree's parent map and the delegating-child map, both
// recorded while align walks the tree. The delegating child of a node is the child its position
// delegates to; the gates consult it to decide whether a node's first token belongs to generated
// code.
type treeRelations struct {
	parents    map[ast.Node]ast.Node
	delegating map[ast.Node]ast.Node
}

// align structurally matches the restored AST with the re-parsed printed output, so that the
// effective position of every restored node can be read from its printed counterpart. Children
// are visited in field declaration order. Import specs are matched by name and path because
// [go/format] sorts them, and deduplicated by name and path because it drops duplicate blank
// imports. Parentheses are unwrapped on either side, because the printer removes redundant
// parentheses (around control clauses and doubled pairs) and adds parentheses for precedence, so
// the two trees can model the same code with different nesting. Comments and empty statements
// are skipped: the former are decorations in dst, and the printer drops the latter. The printed
// tree's parents and delegating children are recorded along the way, so no separate walk of the
// tree is needed.
func align(a ast.Node, b ast.Node, rel *treeRelations) ([]alignedPair, error) {
	for {
		if p, ok := a.(*ast.ParenExpr); ok {
			if _, ok := b.(*ast.ParenExpr); !ok {
				a = p.X
				continue
			}
		}
		if p, ok := b.(*ast.ParenExpr); ok {
			if _, ok := a.(*ast.ParenExpr); !ok {
				b = p.X
				continue
			}
		}
		break
	}
	if reflect.TypeOf(a) != reflect.TypeOf(b) {
		return nil, &alignError{a: a, b: b}
	}
	pairs := []alignedPair{{a, b}}
	childrenA, childrenB := astChildren(a), astChildren(b)
	if len(childrenA) != len(childrenB) {
		return nil, &alignError{a: a, b: b, children: true}
	}
	for i := range childrenA {
		rel.parents[childrenB[i]] = b
		if childrenB[i].Pos() == b.Pos() {
			rel.delegating[b] = childrenB[i]
		}
		child, err := align(childrenA[i], childrenB[i], rel)
		if err != nil {
			return nil, err
		}
		pairs = append(pairs, child...)
	}
	return pairs, nil
}

type alignError struct {
	a, b     ast.Node
	children bool
}

func (e *alignError) Error() string {
	if e.children {
		return "printed AST shape changed under " + fmtNode(e.a)
	}
	return "printed AST node kind changed: " + fmtNode(e.a) + " vs " + fmtNode(e.b)
}

func fmtNode(n ast.Node) string {
	return reflect.TypeOf(n).String()
}

// alignedPair associates a node of the restored AST with its counterpart in the printed output.
type alignedPair struct {
	restored ast.Node
	printed  ast.Node
}

// astChildren returns the AST children of a node in a deterministic order. [ast.File] and import
// declarations are special-cased to skip derived fields, to sort import specs, and to drop
// duplicate import specs, mirroring what [go/format] does to the printed output.
func astChildren(n ast.Node) []ast.Node {
	if file, ok := n.(*ast.File); ok {
		children := make([]ast.Node, 0, len(file.Decls)+1)
		children = append(children, file.Name)
		for _, decl := range file.Decls {
			children = append(children, decl)
		}
		return children
	}
	if decl, ok := n.(*ast.GenDecl); ok && decl.Tok == token.IMPORT {
		specs := slices.Clone(decl.Specs)
		slices.SortFunc(specs, func(x, y ast.Spec) int {
			return strings.Compare(importSpecKey(x.(*ast.ImportSpec)), importSpecKey(y.(*ast.ImportSpec)))
		})
		specs = slices.CompactFunc(specs, func(x, y ast.Spec) bool {
			return importSpecKey(x.(*ast.ImportSpec)) == importSpecKey(y.(*ast.ImportSpec))
		})
		children := make([]ast.Node, len(specs))
		for i, spec := range specs {
			children[i] = spec
		}
		return children
	}

	v := reflect.ValueOf(n)
	if v.Kind() != reflect.Pointer || v.IsNil() {
		return nil
	}
	v = v.Elem()

	var children []ast.Node
	for i := range v.NumField() {
		field := v.Field(i)
		switch field.Kind() {
		case reflect.Pointer, reflect.Interface:
			children = appendNode(children, field)
		case reflect.Slice:
			for j := range field.Len() {
				children = appendNode(children, field.Index(j))
			}
		}
	}
	return children
}

func appendNode(children []ast.Node, value reflect.Value) []ast.Node {
	if !value.IsValid() || value.IsNil() {
		return children
	}
	node, ok := value.Interface().(ast.Node)
	if !ok || node == nil {
		return children
	}
	switch node.(type) {
	case *ast.Comment, *ast.CommentGroup, *ast.EmptyStmt:
		return children
	}
	return append(children, node)
}

func importSpecKey(spec *ast.ImportSpec) string {
	if spec.Name != nil {
		return spec.Name.Name + " " + spec.Path.Value
	}
	return spec.Path.Value
}

// astParents maps every node of a tree to its parent node.
func astParents(root ast.Node) map[ast.Node]ast.Node {
	parents := make(map[ast.Node]ast.Node, 512)
	var visit func(ast.Node)
	visit = func(n ast.Node) {
		for _, child := range astChildren(n) {
			parents[child] = n
			visit(child)
		}
	}
	visit(root)
	return parents
}

// startsAtOriginalToken reports whether the first token of a node belongs to original code: some
// node in the chain of children the position delegates to is original. A generated wrapper
// around an original operand then follows the operand's line, exactly as the previous writer let
// it.
func startsAtOriginalToken(printed ast.Node, resolveOriginal func(ast.Node) bool, delegatingChild func(ast.Node) ast.Node) bool {
	for node := printed; node != nil; {
		if resolveOriginal(node) {
			return true
		}
		node = delegatingChild(node)
	}
	return false
}

// firstTokenGenerated reports whether the first token of a printed node belongs to generated
// code: the node owns no token of its own, and the child its position delegates to is generated.
// A node that merely prints after a generated region without an intervening directive inherits the
// generated attribution, but its own first token is still original and must be re-anchored.
func firstTokenGenerated(printed ast.Node, resolveOriginal func(ast.Node) bool, delegatingChild func(ast.Node) ast.Node) bool {
	if child := delegatingChild(printed); child != nil {
		return !resolveOriginal(child)
	}
	return false
}
