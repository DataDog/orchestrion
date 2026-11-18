// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

//go:build e2e

package e2e_test

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// appendArgsNames is the finite, deterministic name dimension of the #949
// hygiene property (H15): caller bindings named after every identifier the
// rewrite could plausibly need, crossed with parameter and local bindings, a
// receiver binding where legal, integer shadows, callable poison functions for
// the helper names, package-scope declarations, and the import alias
// dimensions (default, explicit, user-generated-looking, shadowed, with a
// file-wide collision beside an original reference).
var appendArgsNames = []string{
	"opts", "append", "slices", "len", "cap", "make", "copy", "nil", "true",
	"__orchestrion_slices", "__orchestrion_lib", "__orchestrion_nameslib",
}

// appendArgsSuffix returns the case-function suffix of a raw name.
func appendArgsSuffix(name string) string {
	return strings.TrimPrefix(name, "__")
}

// appendArgsReceiverNames are the names that also get a receiver-binding
// variant.
var appendArgsReceiverNames = map[string]bool{"opts": true, "slices": true}

// appendArgsDims are the import-spelling dimensions; each dimension gets its
// own file of case functions.
var appendArgsDims = []string{"default", "alias", "useralias", "shadow"}

// testAppendArgsNameMatrix generates the property module, weaves it through
// the real orchestrion binary, and runs its package-scoped tests.
func testAppendArgsNameMatrix(t *testing.T, orchestrionBin string) {
	t.Helper()

	repoRoot, err := findRepoRootForAppendArgs()
	require.NoError(t, err)

	workBase := filepath.Join(repoRoot, "test", "e2e", "work")
	require.NoError(t, os.MkdirAll(workBase, 0o755))
	workDir := filepath.Join(workBase,
		fmt.Sprintf("%s-%s", strings.ReplaceAll(t.Name(), "/", "-"), time.Now().Format("2006-01-02-15-04-05")))
	require.NoError(t, os.MkdirAll(workDir, 0o755))
	t.Cleanup(func() {
		if !t.Failed() {
			os.RemoveAll(workDir)
		} else {
			t.Logf("generated module preserved: %s", workDir)
		}
	})

	writeNameMatrixModule(t, workDir, repoRoot)

	cmd := exec.Command(orchestrionBin, "go", "test", "-count=1", "./names/")
	cmd.Dir = workDir
	cmd.Env = appendArgsEnv()
	output, err := cmd.CombinedOutput()
	require.NoError(t, err, "woven name-matrix tests failed; module preserved at %s:\n%s", workDir, output)
}

const nameMatrixHeader = `// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.
`

// writeNameMatrixModule emits the deterministic source of the property module.
func writeNameMatrixModule(t *testing.T, dir, repoRoot string) {
	t.Helper()

	write := func(rel string, content string) {
		path := filepath.Join(dir, rel)
		require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
		require.NoError(t, os.WriteFile(path, []byte(content), 0o644))
	}

	write("go.mod", fmt.Sprintf("module example.com/appendargs-names\n\ngo 1.26.0\n\nreplace github.com/DataDog/orchestrion => %s\n", repoRoot))

	write("orchestrion.tool.go", nameMatrixHeader+"\n//go:build tools\n\npackage tools\n")

	write("nameslib/nameslib.go", nameMatrixHeader+`package nameslib

import (
	"fmt"
	"sync"
)

type Option struct{ Name string }

var (
	mu    sync.Mutex
	calls [][]string
)

// Record returns the option names recorded by the sink calls.
func Record() [][]string {
	mu.Lock()
	defer mu.Unlock()
	out := make([][]string, len(calls))
	copy(out, calls)
	return out
}

// Reset clears the record.
func Reset() {
	mu.Lock()
	defer mu.Unlock()
	calls = nil
}

// SinkRecord records the names of the options a sink received.
func SinkRecord(opts []Option) {
	mu.Lock()
	defer mu.Unlock()
	names := make([]string, len(opts))
	for i, opt := range opts {
		names[i] = opt.Name
	}
	calls = append(calls, names)
}

func Named(name string) Option { return Option{Name: name} }

// LenStrings returns the length of a string slice, LenCalls the length of a
// recorded call list, and EqStrings compares two string slices. The names
// package shadows the len builtin with a poison function, so its tests must
// use these helpers instead.
func LenStrings(xs []string) int { return len(xs) }

// LenCalls returns the number of recorded sink calls.
func LenCalls(xs [][]string) int { return len(xs) }

// EqStrings reports whether two string slices are equal.
func EqStrings(xs, ys []string) bool {
	if len(xs) != len(ys) {
		return false
	}
	for i := range xs {
		if xs[i] != ys[i] {
			return false
		}
	}
	return true
}

// Capture records the caller value it reads and returns it as an option, so
// the recorded sink call reveals which binding the injected value captured.
func Capture(v any) Option { return Option{Name: fmt.Sprint(v)} }
`)

	// Sinks, package-scope declarations, and the poison functions.
	var sinks strings.Builder
	sinks.WriteString(nameMatrixHeader + "package names\n\nimport \"example.com/appendargs-names/nameslib\"\n\n")
	sinks.WriteString(`// Package-scope declarations occupy alias candidates (H24 dimension).
var (
	slicesVar      = 1 // "slices"
	optsVar        = 2 // "opts"
	genSlicesVar   = 3 // "__orchestrion_slices"
	genLibVar      = 4 // "__orchestrion_lib"
	genNlVar       = 5 // "__orchestrion_nl"
	genNameslibVar = 6 // "__orchestrion_nameslib"
)

// PackageValues exposes the package-scope values for the assertions.
func PackageValues() []int {
	return []int{slicesVar, optsVar, genSlicesVar, genLibVar, genNlVar, genNameslibVar}
}

// The callable poison functions for the helper names, each with a counter.
var poisonAppend, poisonLen, poisonCap, poisonMake, poisonCopy int

// PoisonCounts exposes the poison counters.
func PoisonCounts() []int { return []int{poisonAppend, poisonLen, poisonCap, poisonMake, poisonCopy} }

func append(opts ...nameslib.Option) []nameslib.Option { poisonAppend++; return opts }
func len(xs ...nameslib.Option) int                    { poisonLen++; return 0 }
func cap(xs ...nameslib.Option) int                    { poisonCap++; return 0 }
func make(xs ...nameslib.Option) []nameslib.Option     { poisonMake++; return nil }
func copy(dst, src []nameslib.Option) int              { poisonCopy++; return 0 }

`)
	for _, name := range appendArgsNames {
		sinks.WriteString(fmt.Sprintf("func sink_%s(opts ...nameslib.Option) { nameslib.SinkRecord(opts) }\n", appendArgsSuffix(name)))
	}
	write("names/sinks.go", sinks.String())

	// Case functions: one file per import-spelling dimension of the value
	// package. The shadow dimension shadows the file's own alias at every join
	// point and keeps an original reference beside an occupied suffix
	// candidate.
	emitCases := func(rel, dim, importDecl, alias string, shadowAlias bool) {
		var b strings.Builder
		b.WriteString(nameMatrixHeader + "package names\n\n" + importDecl + "\n\n")
		shadow := ""
		if shadowAlias {
			// The alias shadow stays in scope at the join point; the discard only
			// keeps the original source valid.
			shadow = fmt.Sprintf("\t%s := \"shadow\"\n\t_ = %s\n", alias, alias)
		}
		for _, name := range appendArgsNames {
			suffix := appendArgsSuffix(name)
			// Parameter variant: an integer shadow of the name.
			b.WriteString(fmt.Sprintf("func Case_%s_Param_%s(%s int, args []%s.Option) {\n%s\tsink_%s(args...)\n}\n\n",
				suffix, dim, name, alias, shadow, suffix))
			// Local variant: an outer binding and an inner block binding.
			b.WriteString(fmt.Sprintf("func Case_%s_Local_%s(outer string, args []%s.Option) {\n\t%s := \"local-value\"\n%s\tsink_%s(args...)\n\t_, _ = %s, outer\n}\n\n",
				suffix, dim, alias, name, shadow, suffix, name))
			// Receiver variant where legal.
			if appendArgsReceiverNames[name] {
				b.WriteString(fmt.Sprintf("type %sReceiver%s string\n\nfunc (%s %sReceiver%s) Case(args []%s.Option) {\n%s\tsink_%s(args...)\n}\n\n",
					suffix, dim, name, suffix, dim, alias, shadow, suffix))
			}
		}
		if shadowAlias {
			// An original reference of the value package beside a local that
			// occupies the first suffix candidate: the alias must be safe at
			// every reference of the file, not only at the join points.
			b.WriteString(fmt.Sprintf("func helperShadow() %s.Option {\n\t__orchestrion_nl1 := 1\n\t_ = __orchestrion_nl1\n\treturn %s.Named(\"helper\")\n}\n", alias, alias))
		}
		write(rel, b.String())
	}

	emitCases("names/cases_default.go", "default",
		"import \"example.com/appendargs-names/nameslib\"", "nameslib", false)
	emitCases("names/cases_alias.go", "alias",
		"import nlx \"example.com/appendargs-names/nameslib\"", "nlx", false)
	emitCases("names/cases_useralias.go", "useralias",
		"import __orchestrion_nl \"example.com/appendargs-names/nameslib\"", "__orchestrion_nl", false)
	emitCases("names/cases_shadow.go", "shadow",
		"import nlx \"example.com/appendargs-names/nameslib\"", "nlx", true)

	// The package-scoped tests: every case must append the captured caller
	// value, and neither the poison functions nor the package values may be
	// affected.
	var test strings.Builder
	test.WriteString(nameMatrixHeader + "package names\n\nimport (\n\t\"testing\"\n\n\t\"example.com/appendargs-names/nameslib\"\n)\n\n")
	test.WriteString("func TestNameMatrix(t *testing.T) {\n\tpoisonBefore := PoisonCounts()\n\tpkgBefore := PackageValues()\n\n\tfor _, suffix := range []string{")
	for i, name := range appendArgsNames {
		if i > 0 {
			test.WriteString(", ")
		}
		test.WriteString(fmt.Sprintf("%q", appendArgsSuffix(name)))
	}
	test.WriteString("} {\n\t\tfor _, dim := range []string{")
	for i, dim := range appendArgsDims {
		if i > 0 {
			test.WriteString(", ")
		}
		test.WriteString(fmt.Sprintf("%q", dim))
	}
	test.WriteString("} {\n\t\t\tx := nameslib.Named(\"x\")\n\n")
	test.WriteString(`			invokeParameter(t, suffix, dim, x)
			invokeLocal(t, suffix, dim, x)
			if recv, ok := receiverOf(suffix, dim); ok {
				invokeReceiver(t, suffix, dim, recv, x)
			}
		}
	}

	after := PoisonCounts()
	for i, got := range after {
		if got != poisonBefore[i] {
			t.Fatalf("poison counter %d = %d, want %d", i, got, poisonBefore[i])
		}
	}
	pkgAfter := PackageValues()
	for i, got := range pkgAfter {
		if got != pkgBefore[i] {
			t.Fatalf("package value %d = %d, want %d", i, got, pkgBefore[i])
		}
	}
}

// assertLast asserts the names of the single recorded sink call.
func assertLast(t *testing.T, want ...string) {
	t.Helper()
	calls := nameslib.Record()
	if nameslib.LenCalls(calls) != 1 {
		t.Fatalf("recorded %d calls, want 1", nameslib.LenCalls(calls))
	}
	got := calls[0]
	if !nameslib.EqStrings(got, want) {
		t.Fatalf("recorded %v, want %v", got, want)
	}
}

// invokeParameter runs a parameter variant and asserts the recorded values.
func invokeParameter(t *testing.T, suffix, dim string, x nameslib.Option) {
	t.Helper()
	nameslib.Reset()
	switch suffix + "/" + dim {
`)
	for _, name := range appendArgsNames {
		suffix := appendArgsSuffix(name)
		for _, dim := range appendArgsDims {
			test.WriteString(fmt.Sprintf("\tcase %q:\n\t\tCase_%s_Param_%s(1, []nameslib.Option{x})\n",
				suffix+"/"+dim, suffix, dim))
		}
	}
	test.WriteString(`	default:
		t.Fatalf("unknown suffix %q", suffix)
	}
	assertLast(t, "x", "1")
}

// invokeLocal runs a local variant and asserts the recorded values.
func invokeLocal(t *testing.T, suffix, dim string, x nameslib.Option) {
	t.Helper()
	nameslib.Reset()
	switch suffix + "/" + dim {
`)
	for _, name := range appendArgsNames {
		suffix := appendArgsSuffix(name)
		for _, dim := range appendArgsDims {
			test.WriteString(fmt.Sprintf("\tcase %q:\n\t\tCase_%s_Local_%s(\"outer-value\", []nameslib.Option{x})\n",
				suffix+"/"+dim, suffix, dim))
		}
	}
	test.WriteString(`	default:
		t.Fatalf("unknown suffix %q", suffix)
	}
	assertLast(t, "x", "local-value")
}

// invokeReceiver runs a receiver variant and asserts the recorded values.
func invokeReceiver(t *testing.T, suffix, dim string, recv any, x nameslib.Option) {
	t.Helper()
	nameslib.Reset()
	switch suffix + "/" + dim {
`)
	for _, name := range appendArgsNames {
		if !appendArgsReceiverNames[name] {
			continue
		}
		suffix := appendArgsSuffix(name)
		for _, dim := range appendArgsDims {
			test.WriteString(fmt.Sprintf("\tcase %q:\n\t\trecv.(%sReceiver%s).Case([]nameslib.Option{x})\n",
				suffix+"/"+dim, suffix, dim))
		}
	}
	test.WriteString(`	default:
		t.Fatalf("unknown receiver suffix %q", suffix)
	}
	assertLast(t, "x", "recv-value")
}

// receiverOf returns the receiver value for the receiver variants.
func receiverOf(suffix, dim string) (any, bool) {
	switch suffix + "/" + dim {
`)
	for _, name := range appendArgsNames {
		if !appendArgsReceiverNames[name] {
			continue
		}
		suffix := appendArgsSuffix(name)
		for _, dim := range appendArgsDims {
			test.WriteString(fmt.Sprintf("\tcase %q:\n\t\treturn %sReceiver%s(\"recv-value\"), true\n",
				suffix+"/"+dim, suffix, dim))
		}
	}
	test.WriteString(`	}
	return nil, false
}
`)
	write("names/names_test.go", test.String())

	// The aspect configuration: one aspect per name, each reading its own
	// caller binding through the injected value.
	var yml strings.Builder
	yml.WriteString("%YAML 1.1\n---\nmeta:\n  name: append-args-names\n  description: |-\n    Name-matrix property module for hygienic append-args weaving.\n\naspects:\n")
	for _, name := range appendArgsNames {
		suffix := strings.TrimPrefix(name, "__")
		yml.WriteString(fmt.Sprintf("  - id: append-args/sink.%s\n    join-point:\n      function-call: sink_%s\n    advice:\n      - append-args:\n          type: example.com/appendargs-names/nameslib.Option\n          values:\n            - imports:\n                nl: example.com/appendargs-names/nameslib\n              template: nl.Capture(%s)\n\n", suffix, suffix, name))
	}
	write("orchestrion.yml", yml.String())
}
