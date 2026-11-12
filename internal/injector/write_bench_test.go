// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package injector

import (
	"context"
	"fmt"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/DataDog/orchestrion/internal/injector/aspect"
	injectparse "github.com/DataDog/orchestrion/internal/injector/parse"
	"github.com/DataDog/orchestrion/internal/yaml"
	"github.com/dave/dst"
	"github.com/dave/dst/decorator"
	"github.com/dave/dst/decorator/resolver/gotypes"
	"github.com/stretchr/testify/require"
)

// benchInput writes the benchmark source into its own module and returns the input path and the
// module directory. The source is about 10,000 lines: a woven function followed by 1,200 short
// functions.
func benchInput(b *testing.B) (input string, tmp string) {
	b.Helper()

	tmp = b.TempDir()
	require.NoError(b, os.WriteFile(filepath.Join(tmp, "go.mod"), []byte("module dummy/test/module\n\ngo 1.26\n"), 0o644))

	var src strings.Builder
	src.WriteString("package p\n\nfunc woven() {\n\tprobe()\n}\n\n")
	for i := range 1200 {
		fmt.Fprintf(&src, "func f%d() int {\n\tif ready {\n\t\treturn probe()\n\t}\n\treturn 0\n}\n\n", i)
	}
	src.WriteString("var ready = true\n\n")
	src.WriteString(lineInfoHelpers)

	input = filepath.Join(tmp, "input.go")
	require.NoError(b, os.WriteFile(input, []byte(src.String()), 0o644))
	return input, tmp
}

// benchAspects parses the aspect configuration the benchmarks weave with.
func benchAspects(b *testing.B) []*aspect.Aspect {
	b.Helper()

	var cfg struct {
		Aspects []*aspect.Aspect `yaml:"aspects"`
	}
	require.NoError(b, yaml.UnmarshalContext(context.Background(), strings.NewReader(prependBodyAspects("woven")), &cfg))
	require.NotEmpty(b, cfg.Aspects, "the benchmark must configure aspects")
	return cfg.Aspects
}

// benchPipeline runs parse, type-check, decoration, and advice application for one iteration, and
// returns the decorator and the modified tree. The write step mutates the tree with line
// directives, so the pipeline must run fresh for every iteration.
func benchPipeline(b *testing.B, input string, inj *Injector, aspects []*aspect.Aspect) (*decorator.Decorator, *dst.File) {
	b.Helper()
	ctx := b.Context()

	fset := token.NewFileSet()
	files, err := injectparse.NewParser(fset, 1).ParseFiles(ctx, []string{input}, aspects)
	require.NoError(b, err)
	checked, err := inj.typeCheck(ctx, fset, files)
	require.NoError(b, err)
	dec := decorator.NewDecoratorWithImports(fset, inj.ImportPath, gotypes.New(checked.Uses))
	dstFile, err := dec.DecorateFile(files[0].AstFile)
	require.NoError(b, err)
	_, err = inj.applyAspects(ctx, parameters{
		Decorator: dec,
		File:      dstFile,
		TypeInfo:  checked.Info,
		Importer:  checked.Importer,
		Aspects:   files[0].Aspects,
	})
	require.NoError(b, err)
	return dec, dstFile
}

// BenchmarkWriteStepLargeFile measures the cost of writing one modified file of about 10,000
// lines: import canonicalization, line annotation, restoration, formatting, and the write to
// disk. The parse, type-check, decoration, and advice steps that produce the modified tree are
// outside the measured section, so the benchmark tracks the write step that the line-information
// work changed. BenchmarkInjectionSetupLargeFile measures those outside steps, so the ratio of
// the write step to the rest of the injection work is known.
func BenchmarkWriteStepLargeFile(b *testing.B) {
	input, tmp := benchInput(b)
	aspects := benchAspects(b)
	inj := Injector{
		ImportPath:   "dummy/test/module",
		Lookup:       lookupPackageIn(tmp),
		ModifiedFile: func(path string) string { return filepath.Join(filepath.Dir(path), filepath.Base(path)+".edited.go") },
	}
	require.NoError(b, inj.validate(), "the benchmark must initialize the injector")

	b.ResetTimer()
	for range b.N {
		b.StopTimer()
		dec, dstFile := benchPipeline(b, input, &inj, aspects)
		b.StartTimer()

		if _, err := inj.writeModifiedFile(context.Background(), dec, dstFile); err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkInjectionSetupLargeFile measures the part of the pipeline that the write-step
// benchmark keeps outside its timer: parsing, type-checking, decoration, and advice application
// on the same about-10,000-line source.
func BenchmarkInjectionSetupLargeFile(b *testing.B) {
	input, tmp := benchInput(b)
	aspects := benchAspects(b)
	inj := Injector{
		ImportPath: "dummy/test/module",
		Lookup:     lookupPackageIn(tmp),
	}
	require.NoError(b, inj.validate(), "the benchmark must initialize the injector")

	b.ResetTimer()
	for range b.N {
		benchPipeline(b, input, &inj, aspects)
	}
}
