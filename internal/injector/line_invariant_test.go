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
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/DataDog/orchestrion/internal/injector/aspect"
	injectparse "github.com/DataDog/orchestrion/internal/injector/parse"
	"github.com/dave/dst"
	"github.com/dave/dst/decorator"
	"github.com/dave/dst/decorator/resolver/gotypes"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// generatedFilename is the pseudo-file name the injector attributes to nodes it created.
const generatedFilename = "<generated>"

// AssertFileLineInvariant replays the injection of a single file and verifies that the bytes that
// were actually written preserve the effective source positions of every retained original node.
//
// The verification uses node identity through the decorator and restorer maps, never source text
// matching. It gates:
//   - the adjusted filename and line of every retained original node's position;
//   - the tokens the compiler records for line information: [ast.CallExpr.Lparen], the operator of
//     [ast.AssignStmt], and the [ast.BlockStmt.Rbrace] of function bodies;
//   - that generated statements, declarations, specs, and fields map to [generatedFilename].
//
// An original node whose first token was replaced by generated code (for example an [ast.ExprStmt]
// whose expression was wrapped) is checked through its first original descendant and its own
// tokens instead of its position, as its position is legitimately generated.
//
// It also verifies that the replay writes the exact candidate bytes, that a second restoration is
// byte-stable, that removed original nodes are explained by join-point matches, and that every
// `//line` directive in the output starts in column one.
//
// writtenAs is the path the candidate was written to. The output is re-parsed under that path,
// like the compiler does, so code without a directive shows the written file's name and fails the
// invariant instead of passing under the input file's name.
func AssertFileLineInvariant(t *testing.T, inj Injector, check FileLineCheck) {
	t.Helper()
	ctx := t.Context()

	require.NoError(t, inj.validate(), "invalid injector")
	aspects := inj.packageFilterAspects(check.Aspects)

	fset := token.NewFileSet()
	files, err := injectparse.NewParser(fset, 1).ParseFiles(ctx, []string{check.Input}, aspects)
	require.NoError(t, err, "failed to parse input file")

	if len(files) == 0 {
		// No aspect may match this file, so it must have been written unchanged.
		original, err := os.ReadFile(check.Input)
		require.NoError(t, err)
		assert.Equal(t, string(original), string(check.Written), "file with no applicable aspects must be written unchanged")
		return
	}

	checked, err := inj.typeCheck(ctx, fset, files)
	require.NoError(t, err, "failed to type-check input file")

	for _, parsedFile := range files {
		if len(parsedFile.Aspects) == 0 {
			continue
		}
		assertFileLineInvariant(t, &inj, &lineInvariant{
			input:     check.Input,
			fset:      fset,
			parsed:    parsedFile,
			checked:   checked,
			originals: originalNodeLedger(parsedFile.AstFile),
			parents:   astParentMap(parsedFile.AstFile),
			written:   check.Written,
			writtenAs: check.WrittenAs,
		})
	}
}

// FileLineCheck carries the inputs of [AssertFileLineInvariant].
type FileLineCheck struct {
	// Input is the path of the injected source file.
	Input string
	// Aspects are the aspects the file was injected with.
	Aspects []*aspect.Aspect
	// Written is the bytes of the candidate under test.
	Written []byte
	// WrittenAs is the path the candidate was written to.
	WrittenAs string
}

// lineInvariant carries the replay state shared by the gates below.
type lineInvariant struct {
	input     string           // the path of the injected source file
	fset      *token.FileSet   // the file set the input was parsed in
	parsed    injectparse.File // the parsed input file (pre-advice)
	checked   typeCheckResult
	originals map[ast.Node]bool // every original node of the pre-advice AST
	parents   map[ast.Node]ast.Node
	written   []byte // the bytes of the candidate under test
	writtenAs string // the path the candidate was written to

	genFile    string                // the effective file name the <generated> directive resolves to
	outFset    *token.FileSet        // the file set the written bytes were re-parsed in
	outParents map[ast.Node]ast.Node // the parent map of the re-parsed output
	generated  map[ast.Node]bool     // printed node -> generated (true) or original (false)
}

// originalNodeLedger records every node of the pre-advice AST. Comments carry no source identity
// through the injector (they become dst decorations), so they are excluded from the ledger.
func originalNodeLedger(file *ast.File) map[ast.Node]bool {
	originals := make(map[ast.Node]bool, 512)
	ast.Inspect(file, func(node ast.Node) bool {
		if node != nil {
			switch node.(type) {
			case *ast.Comment, *ast.CommentGroup:
				return false
			}
			originals[node] = true
		}
		return true
	})
	return originals
}

func assertFileLineInvariant(t *testing.T, inj *Injector, inv *lineInvariant) {
	t.Helper()
	input, fset, parsedFile, checked, written := inv.input, inv.fset, inv.parsed, inv.checked, inv.written
	originals := inv.originals

	dec := decorator.NewDecoratorWithImports(fset, inj.ImportPath, gotypes.New(checked.Uses))
	dstFile, err := dec.DecorateFile(parsedFile.AstFile)
	require.NoError(t, err, "failed to decorate input file")

	res, err := inj.injectFile(t.Context(), dec, dstFile, checked, parsedFile.Aspects)
	require.NoError(t, err, "failed to inject file")

	if !res.Modified {
		// The aspect did not modify the file after all: it must have been written unchanged.
		original, err := os.ReadFile(input)
		require.NoError(t, err)
		assert.Equal(t, string(original), string(written), "unmodified file must be written unchanged")
		return
	}

	replayed, err := os.ReadFile(res.Filename)
	require.NoError(t, err)
	require.True(t, bytes.Equal(replayed, written),
		"the provenance replay must write the same bytes as the candidate under test")

	// A second restoration of the annotated tree must not change the artifact.
	second, err := restoreAndFormat(inj, input, dstFile)
	require.NoError(t, err, "failed to restore annotated tree a second time")
	assert.Equal(t, string(postProcess(second)), string(written), "a second restoration must not change the artifact")

	outFset := token.NewFileSet()
	outFile, err := parser.ParseFile(outFset, inv.writtenAs, written, parser.ParseComments)
	require.NoError(t, err, "the written output must parse")
	inv.outParents = astParentMap(outFile)
	inv.generated = make(map[ast.Node]bool, 1024)

	restorer := inj.newRestorer(input)
	restored, err := restorer.RestoreFile(dstFile)
	require.NoError(t, err, "failed to restore annotated tree")

	pairs, err := alignAST(restored, outFile, "")
	require.NoError(t, err, "the restored and printed trees must align structurally")
	restoredParents := astParentMap(restored)

	assertLineDirectivesAtColumnOne(t, written)
	assertGeneratedDirectiveEconomy(t, written)
	inv.genFile = directiveFilename(inv.writtenAs, generatedFilename)
	inv.outFset = outFset

	var (
		originalCount  int
		generatedCount int
		layoutDiffs    int
	)
	for _, pair := range pairs {
		dstNode := restorer.Dst.Nodes[pair.restored]
		if dstNode == nil {
			// Not all synthesized AST nodes are mapped to dst; they carry no source identity.
			continue
		}

		// Diagnostic only: physical layout changes between the measured and printed trees.
		for _, endpoint := range [][2]ast.Node{{pair.restored, pair.printed}} {
			if restorer.Fset.PositionFor(endpoint[0].Pos(), false).Line != outFset.PositionFor(endpoint[1].Pos(), false).Line ||
				restorer.Fset.PositionFor(endpoint[0].End(), false).Line != outFset.PositionFor(endpoint[1].End(), false).Line {
				layoutDiffs++
			}
		}

		org := dec.Ast.Nodes[dstNode]
		if org == nil {
			// The restorer synthesizes dst nodes to expand a path-qualified identifier that
			// the decorator collapsed (e.g. the operands of `errors.New`): the expansion
			// components belong to the original selector they were expanded from.
			org = selectorExpansionOf(pair.restored, restoredParents, restorer, dec)
		}
		if originals[org] {
			originalCount++
			gateOriginalPosition(t, inv, org, pair.restored, pair.printed)
			gateOwnedTokens(t, fset, outFset, org, pair.printed)
		} else {
			generatedCount++
			gateGeneratedNode(t, inv, pair.printed)
		}
	}

	t.Logf("INVARIANT originals=%d generated=%d physical-layout-diffs=%d", originalCount, generatedCount, layoutDiffs)
	require.Positive(t, originalCount,
		"a modified file must retain at least one original node for the invariant to be meaningful")

	assertRemovedOriginals(t, t.Name(), inv, dec, dstFile)
}

// gateOriginalPosition asserts that the adjusted position of the printed counterpart of an
// original node matches the original node's effective position. Nodes whose position delegates to
// a child that advice replaced with generated code are skipped here; their first original
// descendant and their own tokens are gated instead.
func gateOriginalPosition(t *testing.T, inv *lineInvariant, org ast.Node, restored ast.Node, printed ast.Node) {
	t.Helper()
	genFile, outFset := inv.genFile, inv.outFset

	if block, ok := printed.(*ast.BlockStmt); ok && isBodyBlock(block, inv.outParents) {
		// The opening brace of a function body cannot be re-anchored: a directive line before it
		// would insert a semicolon after the parameter list and break the code. The compiler
		// records no line information for the brace; its function is covered by the declaration
		// and the closing brace is covered by the token gate.
		return
	}

	want := inv.fset.PositionFor(org.Pos(), true)
	got := outFset.PositionFor(printed.Pos(), true)
	if got.Filename == genFile && posDelegatesToChild(org) {
		// The first token of this node is generated (e.g. an [ast.ExprStmt] whose expression was
		// wrapped): its position legitimately maps to generated code.
		return
	}
	if want.Filename != got.Filename || want.Line != got.Line {
		assert.Failf(t, "original node changed effective position",
			"original %T at %s is printed at %s (restored %T at %s)",
			org, want, got, restored, outFset.PositionFor(restored.Pos(), true))
	}
}

// gateOwnedTokens asserts the effective line of the tokens the compiler records for line
// information: the parenthesis of calls, the operator of assignments, and the closing brace of
// function bodies (the implicit return line).
func gateOwnedTokens(t *testing.T, fset *token.FileSet, outFset *token.FileSet, org ast.Node, printed ast.Node) {
	t.Helper()

	gate := func(what string, wantPos, gotPos token.Pos) {
		want := fset.PositionFor(wantPos, true)
		got := outFset.PositionFor(gotPos, true)
		if want.Filename != got.Filename || want.Line != got.Line {
			assert.Failf(t, "owned token changed effective line",
				"%s of original %T at %s is printed at %s", what, org, want, got)
		}
	}

	switch org := org.(type) {
	case *ast.CallExpr:
		if out, ok := printed.(*ast.CallExpr); ok {
			gate("call parenthesis", org.Lparen, out.Lparen)
		}
	case *ast.AssignStmt:
		if out, ok := printed.(*ast.AssignStmt); ok {
			gate("assignment operator", org.TokPos, out.TokPos)
		}
	case *ast.FuncDecl:
		if out, ok := printed.(*ast.FuncDecl); ok && org.Body != nil && out.Body != nil {
			gate("function body closing brace", org.Body.Rbrace, out.Body.Rbrace)
		}
	case *ast.FuncLit:
		if out, ok := printed.(*ast.FuncLit); ok && org.Body != nil && out.Body != nil {
			gate("function literal body closing brace", org.Body.Rbrace, out.Body.Rbrace)
		}
	}
}

// gateGeneratedNode asserts that generated code that the compiler can attribute line information
// to (statements, declarations, specs, and fields) maps to the generated pseudo-file. The compiler
// resolves a relative `//line` file name against the file's directory, so the expected effective
// file name is the joined one.
func gateGeneratedNode(t *testing.T, inv *lineInvariant, printed ast.Node) {
	t.Helper()

	// Generated code is attributed to the generated pseudo-file at every node kind, as the
	// previous writer did. Signatures are the exception: the previous writer left generated
	// signature identifiers on the line they are printed on, they carry no line information of
	// their own, and no directive can be placed inside a signature without splitting it. A
	// generated wrapper around an original operand is the other exception: the previous writer
	// let the wrapper follow the operand's line.
	if inSignature(printed, inv.outParents) {
		return
	}
	if startsAtOriginalToken(printed, inv) {
		return
	}

	if got := inv.outFset.PositionFor(printed.Pos(), true); got.Filename != inv.genFile {
		assert.Failf(t, "generated node attributed to source",
			"generated %T is printed at %s, expected file %s", printed, got, generatedFilename)
	}
}

// directiveFilename returns the file name the compiler resolves a `//line` directive to: relative
// directive file names are joined with the directory of the file they appear in.
func directiveFilename(input string, name string) string {
	dir, _ := filepath.Split(input)
	return filepath.Join(dir, name)
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

// startsAtOriginalToken reports whether the first token of a printed node belongs to original
// code: some node in the chain of children the position delegates to is original.
func startsAtOriginalToken(printed ast.Node, inv *lineInvariant) bool {
	for node := printed; node != nil; {
		if resolveClassified(node, inv) {
			return true
		}
		node = delegatingChild(node)
	}
	return false
}

// resolveClassified reports whether a printed node maps to an original node of the input file.
// Nodes the alignment did not classify inherit their parent's classification: the restorer
// synthesizes some of them without a dst identity, and their identity rides on the nearest
// classified ancestor.
func resolveClassified(node ast.Node, inv *lineInvariant) bool {
	if node == nil {
		return true
	}
	if kind, ok := inv.generated[node]; ok {
		return !kind
	}
	return resolveClassified(inv.outParents[node], inv)
}

// delegatingChild returns the child a node's position delegates to, or nil.
func delegatingChild(n ast.Node) ast.Node {
	for _, child := range astChildren(n) {
		if child.Pos() == n.Pos() {
			return child
		}
	}
	return nil
}

// isBodyBlock reports whether a block statement is the body of a function or of a control
// statement (if, for, range, switch, type switch, select). Its opening brace cannot be
// re-anchored: a directive line before it would insert a semicolon after the header and break
// the code. The brace carries no line information; its declaration or statement is anchored, and
// a function body's closing brace is covered by the token gate.
func isBodyBlock(block *ast.BlockStmt, parents map[ast.Node]ast.Node) bool {
	switch parents[block].(type) {
	case *ast.FuncDecl, *ast.FuncLit, *ast.IfStmt, *ast.ForStmt, *ast.RangeStmt,
		*ast.SwitchStmt, *ast.TypeSwitchStmt, *ast.SelectStmt:
		return true
	}
	return false
}

// inSignature reports whether a node is part of a function signature: the parameters or results
// subtree of a function type.
func inSignature(node ast.Node, parents map[ast.Node]ast.Node) bool {
	for n := node; n != nil; {
		switch p := parents[n]; p.(type) {
		case *ast.FuncType:
			return true
		case *ast.FuncDecl, *ast.FuncLit, *ast.BlockStmt:
			return false
		}
		n = parents[n]
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

// assertRemovedOriginals verifies that every original node that no longer appears in the woven
// tree was removed as part of a subtree replacement at a join-point match, and not lost identity.
func assertRemovedOriginals(t *testing.T, name string, inv *lineInvariant, dec *decorator.Decorator, dstFile *dst.File) {
	t.Helper()
	fset, originals, parents, aspects := inv.fset, inv.originals, inv.parents, inv.parsed.Aspects

	reachable := make(map[dst.Node]bool)
	dst.Walk(&dstCollectVisitor{nodes: reachable}, dstFile)

	// Original AST nodes and the dst nodes that still map to them.
	reverse := make(map[ast.Node][]dst.Node, len(originals))
	for dstNode, org := range dec.Ast.Nodes {
		if originals[org] {
			reverse[org] = append(reverse[org], dstNode)
		}
	}

	removed := make(map[ast.Node]bool, len(originals))
	for org := range originals {
		if len(reverse[org]) == 0 {
			// The decorator does not map this node to an individual dst node (e.g. the operands of a
			// selector expression, or an [ast.FuncType]): its identity is carried by the mapping of
			// one of its ancestors, so it cannot be lost on its own.
			continue
		}
		retained := false
		for _, dstNode := range reverse[org] {
			if reachable[dstNode] {
				retained = true
				break
			}
		}
		if !retained {
			removed[org] = true
		}
	}
	if len(removed) == 0 {
		return
	}

	allowed := joinPointRootKinds(aspects)
	var roots []string
	for org := range removed {
		if parent := parents[org]; parent != nil && removed[parent] {
			// Inside an already removed subtree; the root is accountable for it.
			continue
		}
		// Import management legitimately drops and renames import specs during canonicalization.
		if _, isImportSpec := org.(*ast.ImportSpec); isImportSpec {
			roots = append(roots, fmt.Sprintf("%T@%d", org, fset.PositionFor(org.Pos(), true).Line))
			continue
		}
		if allowed != nil && !slices.Contains(allowed, reflect.TypeOf(org)) {
			assert.Failf(t, "original node removed outside of a join-point match",
				"%s: original %T at %s was removed without replacement; allowed kinds: %v",
				name, org, fset.PositionFor(org.Pos(), true), allowed)
			continue
		}
		roots = append(roots, fmt.Sprintf("%T@%d", org, fset.PositionFor(org.Pos(), true).Line))
	}
	slices.Sort(roots)
	t.Logf("INVARIANT removed-originals=%d roots=%v", len(removed), roots)
}

// joinPointRootKinds returns the AST node kinds that the configured aspects' join-points can
// match, or nil when the configured join-points are not statically classifiable. Removals rooted
// at these nodes are expected outcomes of advice application.
func joinPointRootKinds(aspects []*aspect.Aspect) []reflect.Type {
	var kinds []reflect.Type
	for _, asp := range aspects {
		// The join point types are unexported; switch on their type name.
		switch fmt.Sprintf("%T", asp.JoinPoint) {
		case "*join.functionCall", "*join.methodCall":
			kinds = append(kinds, reflect.TypeFor[*ast.CallExpr]())
		case "*join.functionBody":
			kinds = append(kinds, reflect.TypeFor[*ast.FuncDecl](), reflect.TypeFor[*ast.FuncLit]())
		case "*join.structDefinition":
			kinds = append(kinds, reflect.TypeFor[*ast.StructType]())
		default:
			// Unknown join-point kinds: be permissive rather than fail on new syntax.
			return nil
		}
	}
	return kinds
}

// assertGeneratedDirectiveEconomy verifies that the output carries at most one `//line
// <generated>` directive per contiguous generated region: two directives that both attribute to
// the generated pseudo-file, without a directive that re-anchors to a source file between them,
// are redundant. The previous writer emitted one directive per region, and this gate keeps the
// rework from regressing that.
func assertGeneratedDirectiveEconomy(t testing.TB, src []byte) {
	t.Helper()

	lastWasGenerated := false
	for lineNum, line := range strings.Split(string(src), "\n") {
		targets := inlineLineDirectiveTarget.FindAllStringSubmatch(line, -1)
		if trimmed := strings.TrimLeft(line, " \t"); strings.HasPrefix(trimmed, "//line ") {
			targets = append(targets, lineLineDirectiveTarget.FindStringSubmatch(trimmed))
		}
		for _, target := range targets {
			if target == nil {
				continue
			}
			isGenerated := target[1] == generatedFilename
			if isGenerated && lastWasGenerated {
				assert.Failf(t, "redundant generated directive",
					"the //line <generated> directive on line %d follows another generated directive with no source directive between them; one directive per contiguous generated region is enough",
					lineNum+1)
			}
			lastWasGenerated = isGenerated
		}
	}
}

var (
	lineLineDirectiveTarget   = regexp.MustCompile(`^//line (.+?):\d+(?::\d+)?\r?$`)
	inlineLineDirectiveTarget = regexp.MustCompile(`/\*line (.+?):\d+(?::\d+)?\*/`)
)

// TestGeneratedDirectiveEconomyWindowsSource verifies that the directive-economy gate recognizes
// drive-qualified source names (like `D:\a\input.go:42`), which contain colons. Windows CI
// runs the fixtures with such paths. The original expressions excluded colons from the captured
// file name and therefore missed the source directive between two generated regions, reporting a
// false duplicate. The eight cases cover both drive separators, both directive forms, and both
// line endings, without depending on native Windows path semantics.
func TestGeneratedDirectiveEconomyWindowsSource(t *testing.T) {
	for _, filename := range []string{`D:\a\orchestrion\input.go`, `D:/a/orchestrion/input.go`} {
		for _, directive := range []string{"//line " + filename + ":42", "/*line " + filename + ":42*/"} {
			for _, newline := range []string{"\n", "\r\n"} {
				t.Run(directive+newline, func(t *testing.T) {
					src := strings.Join([]string{
						"//line <generated>:1", "generated()", directive, "original()",
						"//line <generated>:1", "generatedAgain()",
					}, newline)
					assertGeneratedDirectiveEconomy(t, []byte(src))
				})
			}
		}
	}
}

// recordingTB records the failure calls made on an embedded [testing.TB] so a test can assert
// that an invariant helper rejected a candidate without ending the test on the first failure.
type recordingTB struct {
	testing.TB
	failures []string
}

func (r *recordingTB) Errorf(format string, args ...any) {
	r.failures = append(r.failures, fmt.Sprintf(format, args...))
}

func (r *recordingTB) Fatalf(format string, args ...any) {
	r.failures = append(r.failures, fmt.Sprintf(format, args...))
}

// TestGeneratedDirectiveEconomyDuplicateControl is the negative control for the economy gate: two
// generated directives with no source directive between them are still rejected as redundant
// after the Windows fix, and the rejection is exactly one failure.
func TestGeneratedDirectiveEconomyDuplicateControl(t *testing.T) {
	rec := &recordingTB{TB: t}
	src := []byte("//line <generated>:1\ngenerated()\n//line <generated>:1\ngeneratedAgain()\n")
	assertGeneratedDirectiveEconomy(rec, src)
	require.Len(t, rec.failures, 1, "expected exactly one economy failure, got %v", rec.failures)
	require.Contains(t, rec.failures[0], "redundant generated directive")
}

func assertLineDirectivesAtColumnOne(t *testing.T, src []byte) {
	t.Helper()

	for lineNum, line := range strings.Split(string(src), "\n") {
		trimmed := strings.TrimLeft(line, " \t")
		if len(trimmed) != len(line) && strings.HasPrefix(trimmed, "//line ") {
			assert.Failf(t, "indented line directive",
				"//line directive on line %d starts in column %d, so the compiler ignores it",
				lineNum+1, len(line)-len(trimmed)+1)
		}
	}
}

// restoreAndFormat restores a dst file with a fresh restorer and formats it, without the injector
// writing anything to disk.
func restoreAndFormat(inj *Injector, filename string, file *dst.File) ([]byte, error) {
	restorer := inj.newRestorer(filename)
	astFile, err := restorer.RestoreFile(file)
	if err != nil {
		return nil, err
	}
	var buf bytes.Buffer
	if err := format.Node(&buf, restorer.Fset, astFile); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

type dstCollectVisitor struct {
	nodes map[dst.Node]bool
}

func (v *dstCollectVisitor) Visit(node dst.Node) dst.Visitor {
	if node != nil {
		v.nodes[node] = true
	}
	return v
}

// alignedPair associates a node of the restored AST with its counterpart in the re-parsed printed
// output.
type alignedPair struct {
	restored ast.Node
	printed  ast.Node
}

// alignAST structurally matches two trees of the same shape. Children are visited in field
// declaration order; import specs are matched by name and path because [go/format] sorts them.
// Comments and empty statements are skipped: their placement does not carry source identity.
// Parentheses are unwrapped on either side, because the printer removes redundant parentheses
// (around control clauses and doubled pairs) and adds parentheses for precedence, so the two
// trees can model the same code with different nesting.
func alignAST(a ast.Node, b ast.Node, path string) ([]alignedPair, error) {
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
		return nil, fmt.Errorf("node kind mismatch at %s: %T vs %T", path, a, b)
	}
	pairs := []alignedPair{{a, b}}

	childrenA, childrenB := astChildren(a), astChildren(b)
	if len(childrenA) != len(childrenB) {
		return nil, fmt.Errorf("child count mismatch at %s (%T): %d vs %d", path, a, len(childrenA), len(childrenB))
	}
	for i := range childrenA {
		child, err := alignAST(childrenA[i], childrenB[i], fmt.Sprintf("%s/%T[%d]", path, a, i))
		if err != nil {
			return nil, err
		}
		pairs = append(pairs, child...)
	}
	return pairs, nil
}

// astChildren returns the AST children of a node in a deterministic order. [ast.File] and import
// declarations are special-cased to skip derived fields and to sort import specs.
func astChildren(n ast.Node) []ast.Node {
	if file, ok := n.(*ast.File); ok {
		children := []ast.Node{file.Name}
		for _, decl := range file.Decls {
			children = append(children, decl)
		}
		return children
	}
	if decl, ok := n.(*ast.GenDecl); ok && decl.Tok == token.IMPORT {
		specs := slices.Clone(decl.Specs)
		slices.SortFunc(specs, func(x, y ast.Spec) int {
			left, right := x.(*ast.ImportSpec), y.(*ast.ImportSpec)
			if c := strings.Compare(importSpecName(left), importSpecName(right)); c != 0 {
				return c
			}
			return strings.Compare(left.Path.Value, right.Path.Value)
		})
		// The printer drops duplicate blank imports from grouped blocks, so both sides are
		// deduplicated by (name, path) to stay comparable.
		specs = slices.CompactFunc(specs, func(x, y ast.Spec) bool {
			return importSpecName(x.(*ast.ImportSpec)) == importSpecName(y.(*ast.ImportSpec)) &&
				x.(*ast.ImportSpec).Path.Value == y.(*ast.ImportSpec).Path.Value
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

func importSpecName(spec *ast.ImportSpec) string {
	if spec.Name != nil {
		return spec.Name.Name
	}
	return ""
}

// astParentMap maps every node of a tree to its parent node, following [astChildren].
func astParentMap(root ast.Node) map[ast.Node]ast.Node {
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

// posDelegatesToChild reports whether the position of a node is the position of one of its
// children, meaning the node owns no token of its own (e.g. an [ast.ExprStmt]).
func posDelegatesToChild(n ast.Node) bool {
	for _, child := range astChildren(n) {
		if child.Pos() == n.Pos() {
			return true
		}
	}
	return false
}
