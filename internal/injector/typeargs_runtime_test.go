// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package injector_test

import (
	gocontext "context"
	"fmt"
	"go/parser"
	"go/token"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"

	"github.com/DataDog/orchestrion/internal/injector"
	"github.com/DataDog/orchestrion/internal/injector/aspect"
	"github.com/DataDog/orchestrion/internal/injector/aspect/advice"
	"github.com/DataDog/orchestrion/internal/injector/aspect/advice/code"
	"github.com/DataDog/orchestrion/internal/injector/aspect/context"
	"github.com/DataDog/orchestrion/internal/injector/aspect/join"
	"github.com/DataDog/orchestrion/internal/injector/typed"
	"github.com/stretchr/testify/require"
	"golang.org/x/tools/go/packages"
)

// injectAndRun sets up a temporary module containing src, injects the
// configured aspects into it as the injector does for real compilation units,
// and runs the modified main package. It returns the program's stdout.
func injectAndRun(t *testing.T, src string, aspects []*aspect.Aspect) string {
	t.Helper()

	_, thisFile, _, _ := runtime.Caller(0)
	rootDir := filepath.Join(thisFile, "..", "..", "..")
	tmp := t.TempDir()

	testLookup := func(path string) (io.ReadCloser, error) {
		pkgs, err := packages.Load(
			&packages.Config{Mode: packages.NeedExportFile, Dir: tmp},
			path,
		)
		if err != nil {
			return nil, err
		}
		file := pkgs[0].ExportFile
		if file == "" {
			return nil, fmt.Errorf("no export file found for %q", path)
		}
		return os.Open(file)
	}

	runGo(t, tmp, "mod", "init", testModuleName)
	runGo(t, tmp, "mod", "edit",
		"-replace=github.com/DataDog/orchestrion="+rootDir,
		"-replace=github.com/DataDog/orchestrion/instrument="+filepath.Join(rootDir, "instrument"),
	)

	inputFile := filepath.Join(tmp, "input.go")
	original := strings.TrimSpace(src) + "\n"
	require.NoError(t, os.WriteFile(inputFile, []byte(original), 0o644), "failed to create input.go")
	runGo(t, tmp, "mod", "tidy")

	importMap := make(map[string]string)
	for _, spec := range importsOf(t, original) {
		importMap[spec] = ""
	}

	inj := injector.Injector{
		ModifiedFile: func(path string) string { return filepath.Join(tmp, filepath.Base(path)+".edited.go") },
		ImportPath:   testModuleName,
		Lookup:       testLookup,
		ImportMap:    importMap,
	}

	res, _, err := inj.InjectFiles(gocontext.Background(), []string{inputFile}, aspects)
	require.NoError(t, err, "failed to inject file")

	resFile, modified := res[inputFile]
	require.True(t, modified, "the aspects must have modified the file")
	require.NoError(t, os.Rename(resFile.Filename, inputFile), "failed to replace the input file")
	runGo(t, tmp, "mod", "tidy")

	var stdout strings.Builder
	cmd := exec.Command("go", "run", ".")
	cmd.Dir = tmp
	cmd.Stdout = &stdout
	cmd.Stderr = os.Stderr
	require.NoError(t, cmd.Run(), "failed running the injected program")

	return stdout.String()
}

// importsOf returns the import paths declared by a source string.
func importsOf(t *testing.T, src string) []string {
	t.Helper()

	fset := token.NewFileSet()
	astFile, err := parser.ParseFile(fset, "input.go", src, parser.ParseComments)
	require.NoError(t, err, "failed to parse the source")

	imports := make([]string, 0, len(astFile.Imports))
	for _, spec := range astFile.Imports {
		path, err := strconv.Unquote(spec.Path.Value)
		require.NoError(t, err, "failed to unquote import path %q", spec.Path.Value)
		imports = append(imports, path)
	}
	return imports
}

// TestInjectorTypeArgsRuntime verifies the runtime behavior of advice on
// explicit generic calls (matrix A04, A27): wrapping counts every matched
// call exactly once, and append-args on a spread call evaluates the caller's
// slice expression once, preserves its backing array, and appends the
// configured values. The Connect-shaped call is the issue's motivating
// two-type-argument instantiation with variadic options.
func TestInjectorTypeArgsRuntime(t *testing.T) {
	t.Run("wrap-expression counts explicit calls", func(t *testing.T) {
		src := `package main

import "fmt"

var marks int

func Mark[T any](v T) T { marks++; return v }

func F[A any](a A) A { return a }

func main() {
	_ = F[int](1)
	_ = F(2)
	_ = F[string]("x")
	fmt.Println("marks=", marks)
}
`
		aspects := []*aspect.Aspect{{
			ID:        "mark-explicit-calls",
			JoinPoint: join.FunctionCall("", "F"),
			Advice: []advice.Advice{
				advice.WrapExpression(code.MustTemplate("Mark({{ . }})", nil, context.GoLangVersion{})),
			},
		}}

		require.Equal(t, "marks= 3\n", injectAndRun(t, src, aspects))
	})

	t.Run("append-args preserves spread semantics", func(t *testing.T) {
		src := `package main

import "fmt"

var evals int

var base = []string{"a", "b", "c", "d"}

func mk() []string { evals++; return base[:2] }

func One[A any](url string, opts ...string) int { return len(opts) }
func Two[A, B any](url string, opts ...B) int  { return len(opts) }

func main() {
	r1 := One[int]("u", mk()...)
	r2 := Two[int, string]("u", mk()...)
	r3 := Two[int]("u", mk()...)
	fmt.Println("r1=", r1, "r2=", r2, "r3=", r3, "evals=", evals, "base2=", base[2])
}
`
		aspects := []*aspect.Aspect{{
			ID: "append-option",
			JoinPoint: join.OneOf(
				join.FunctionCall("", "One"),
				join.FunctionCall("", "Two"),
			),
			Advice: []advice.Advice{
				advice.AppendArgs(typed.MustTypeName("string"), code.MustTemplate(`"z"`, nil, context.GoLangVersion{})),
			},
		}}

		require.Equal(
			t,
			"r1= 3 r2= 3 r3= 3 evals= 3 base2= c\n",
			injectAndRun(t, src, aspects),
		)
	})
}
