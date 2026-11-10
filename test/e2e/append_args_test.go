// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

//go:build e2e

package e2e_test

import (
	"context"
	goparser "go/parser"
	"go/token"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/DataDog/orchestrion/test/e2e"
	"github.com/stretchr/testify/require"
)

// appendArgsEnv keeps automatic pinning from adding tracer integrations the
// fixtures do not need; each module provides its own orchestrion.yml and
// orchestrion.tool.go.
func appendArgsEnv() []string {
	return append(os.Environ(), "DD_ORCHESTRION_IS_GOMOD_VERSION=true")
}

// runAppendArgsModule copies the given testdata module and runs one orchestrion
// go command in it, returning the combined output.
func runAppendArgsModule(t *testing.T, bin, module string, args ...string) (string, error) {
	t.Helper()

	workDir := e2e.CreateWorkDir(t, module)

	ctx, cancel := context.WithTimeout(context.Background(), e2e.TestTimeout(t))
	defer cancel()

	cmd := exec.CommandContext(ctx, bin, append([]string{"go"}, args...)...)
	cmd.Dir = workDir
	cmd.Env = appendArgsEnv()

	output, err := cmd.CombinedOutput()
	if writeErr := os.WriteFile(filepath.Join(workDir, "last-run.log"), output, 0o644); writeErr != nil {
		t.Logf("failed to write run log: %v", writeErr)
	}
	return string(output), err
}

// requireGoDirective asserts the module in the work tree still declares the
// given go directive, so the run cannot silently upgrade the module's language.
func requireGoDirective(t *testing.T, module, directive string) {
	t.Helper()

	repoRoot, err := findRepoRootForAppendArgs()
	require.NoError(t, err)
	data, err := os.ReadFile(filepath.Join(repoRoot, "test", "e2e", "testdata", module, "go.mod"))
	require.NoError(t, err)
	if !strings.Contains(string(data), "go "+directive) {
		t.Fatalf("the %s module must declare go %s", module, directive)
	}
}

// TestAppendArgs covers the runtime rows of the #949 matrix: capture,
// hygiene, slice behavior, evaluation order, call forms, composition,
// zero-template no-ops, generics, tuple rejection, and language versions.
// The behavior assertions live in the fixture packages themselves; this test
// builds them through the real orchestrion toolexec pipeline, which is the
// only place that proves runtime binding, import resolution, and race safety.
func TestAppendArgs(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping e2e append-args tests in short mode")
	}

	orchestrionBin := e2e.FindOrchestrionBinary(t)
	t.Logf("Using orchestrion binary: %s", orchestrionBin)

	// One subtest per fixture package of the main module.
	packages := []struct {
		name string
		pkg  string
	}{
		{"Capture", "./capture/"},
		{"Hygiene", "./hygiene/"},
		{"Hygiene/CallableAppend", "./callableappend/"},
		{"Slices", "./slices/"},
		{"Order", "./order/"},
		{"Calls", "./calls/"},
		{"Composition", "./composition/"},
		{"EmptyAdvice", "./emptyadvice/"},
		{"Types", "./types/"},
	}
	for _, p := range packages {
		t.Run(p.name, func(t *testing.T) {
			output, err := runAppendArgsModule(t, orchestrionBin, "testdata/appendargs", "test", "-count=1", p.pkg)
			require.NoError(t, err, "woven package tests failed:\n%s", output)
			requireGoDirective(t, "appendargs", "1.26.0")
		})
	}

	// S08: the concurrent row must run the woven package under the race
	// detector itself.
	t.Run("Slices/Concurrent", func(t *testing.T) {
		output, err := runAppendArgsModule(t, orchestrionBin, "testdata/appendargs",
			"test", "-race", "-count=1", "-run", "^TestSlicesConcurrent$", "./slices/")
		require.NoError(t, err, "woven concurrent test failed under -race:\n%s", output)
	})

	// C05: the woven build of the sole-tuple call fails with the append-args
	// advice error naming the call, the restriction, and the value count.
	t.Run("Calls/TupleDiagnostic", func(t *testing.T) {
		output, err := runAppendArgsModule(t, orchestrionBin, "testdata/appendargs", "build", "./tuple/")
		require.Error(t, err, "the woven tuple build must fail instead of emitting invalid Go")
		for _, want := range []string{"append-arguments", "2 values", "TupleSink"} {
			if !strings.Contains(output, want) {
				t.Fatalf("build output missing %q:\n%s", want, output)
			}
		}
	})

	// V08: no fixture package of the main module imports slices itself; the
	// generated import resolves through the toolexec import configuration,
	// which the package runs above exercised.
	t.Run("Language/NoDirectSlicesImport", func(t *testing.T) {
		require.False(t, hasDirectSlicesImport(t, "appendargs"),
			"no fixture source of the module may import slices directly")
	})

	// V01, V05: a go 1.17 module. The woven app package uses explicit generic
	// instantiation, which the compiler rejects at the go1.17 language level, so
	// its successful compilation is the proof that the minimum-language
	// propagation reached the compiler: `go build -x` cannot show the flag,
	// because go prints the command line before the toolexec proxy rewrites it.
	// The noop package stays at the module's own language, because the no-op
	// advice requests nothing.
	t.Run("Language/Go117", func(t *testing.T) {
		output, err := runAppendArgsModule(t, orchestrionBin, "testdata/appendargs-go117",
			"test", "-count=1", "./app/", "./noop/")
		require.NoError(t, err, "woven go1.17 module tests failed:\n%s", output)
		requireGoDirective(t, "appendargs-go117", "1.17")
	})

	// V02: go 1.18 and go 1.20 modules.
	t.Run("Language/Go118", func(t *testing.T) {
		output, err := runAppendArgsModule(t, orchestrionBin, "testdata/appendargs-go118",
			"test", "-count=1", "./app/")
		require.NoError(t, err, "woven go1.18 module test failed:\n%s", output)
		requireGoDirective(t, "appendargs-go118", "1.18")
	})
	t.Run("Language/Go120", func(t *testing.T) {
		output, err := runAppendArgsModule(t, orchestrionBin, "testdata/appendargs-go120",
			"test", "-count=1", "./app/")
		require.NoError(t, err, "woven go1.20 module test failed:\n%s", output)
		requireGoDirective(t, "appendargs-go120", "1.20")
	})

	// V03: go 1.21 and go 1.22 modules.
	t.Run("Language/Go121", func(t *testing.T) {
		output, err := runAppendArgsModule(t, orchestrionBin, "testdata/appendargs-go121",
			"test", "-count=1", "./app/", "./loopvar/")
		require.NoError(t, err, "woven go1.21 module tests failed:\n%s", output)
		requireGoDirective(t, "appendargs-go121", "1.21")
	})
	t.Run("Language/Go122", func(t *testing.T) {
		output, err := runAppendArgsModule(t, orchestrionBin, "testdata/appendargs-go122",
			"test", "-count=1", "./app/")
		require.NoError(t, err, "woven go1.22 module test failed:\n%s", output)
		requireGoDirective(t, "appendargs-go122", "1.22")
	})

	// V06: under the go1.21 module language, the loop variable addresses stay
	// equal after weaving; the go121 run above contains this assertion in the
	// fixture package, and this subtest names the row for the matrix.
	t.Run("Language/LoopVarGo121", func(t *testing.T) {
		output, err := runAppendArgsModule(t, orchestrionBin, "testdata/appendargs-go121",
			"test", "-count=1", "-run", "^TestLoopVarGo121$", "./loopvar/")
		require.NoError(t, err, "loop-variable semantics changed through weaving:\n%s", output)
		requireGoDirective(t, "appendargs-go121", "1.21")
	})

	// H15: the name-matrix property module.
	t.Run("Hygiene/NameMatrix", func(t *testing.T) {
		testAppendArgsNameMatrix(t, orchestrionBin)
	})
}

// hasDirectSlicesImport reports whether any .go file of the module directory
// tree directly imports slices, parsed from the actual import declarations so
// comments cannot produce false positives.
func hasDirectSlicesImport(t *testing.T, module string) bool {
	t.Helper()

	repoRoot, err := findRepoRootForAppendArgs()
	require.NoError(t, err)
	root := filepath.Join(repoRoot, "test", "e2e", "testdata", module)

	found := false
	err = filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() || !strings.HasSuffix(path, ".go") {
			return err
		}
		fset := token.NewFileSet()
		astFile, parseErr := goparser.ParseFile(fset, path, nil, goparser.ImportsOnly)
		if parseErr != nil {
			return parseErr
		}
		for _, imp := range astFile.Imports {
			if lit, unquoteErr := strconv.Unquote(imp.Path.Value); unquoteErr == nil && lit == "slices" {
				found = true
			}
		}
		return nil
	})
	require.NoError(t, err)
	return found
}
func findRepoRootForAppendArgs() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		data, readErr := os.ReadFile(filepath.Join(dir, "go.mod"))
		if readErr == nil && strings.Contains(string(data), "module github.com/DataDog/orchestrion\n") {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", os.ErrNotExist
		}
		dir = parent
	}
}
