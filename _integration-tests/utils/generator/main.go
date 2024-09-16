// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package main

import (
	"flag"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"log"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"

	"github.com/dave/jennifer/jen"
)

const (
	genTestName = "gen_test.go"
	utilsPkg    = "orchestrion/integration/utils"
)

func init() {
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: %s <dir>\n", os.Args[0])
		flag.PrintDefaults()
	}
}

func main() {
	flag.Parse()
	if flag.NArg() != 1 {
		flag.Usage()
		os.Exit(2)
	}
	root, err := filepath.Abs(flag.Arg(0))
	if err != nil {
		log.Fatal(err)
	}

	testPkgs, err := os.ReadDir(root)
	if err != nil {
		log.Fatalf("failed listing directory: %v\n", err)
	}
	// Ensure stable ordering by explicitly sorting...
	slices.SortFunc(testPkgs, func(lhs, rhs os.DirEntry) int {
		return strings.Compare(lhs.Name(), rhs.Name())
	})

	for _, pkg := range testPkgs {
		if !pkg.IsDir() || pkg.Name() == "testdata" {
			continue
		}
		testDir := path.Join(root, pkg.Name())
		out := path.Join(testDir, genTestName)
		pkgName, testCases := parseCode(testDir)
		f := testFile(pkgName)

		f.
			Func().
			Id("TestIntegration").
			Params(jen.Id("t").Op("*").Qual("testing", "T")).
			Block(
				// testCases := map[string]utils.TestCase{ ... }
				jen.Id("testCases").Op(":=").Map(jen.String()).Qual(utilsPkg, "TestCase").ValuesFunc(func(g *jen.Group) {
					for _, tc := range testCases {
						name := "Main"
						if n := strings.TrimPrefix(tc, "TestCase"); n != "" {
							name = n
						}
						g.Line().Lit(name).Op(":").New(jen.Id(tc))
					}
					g.Line().Empty()
				}),
				// runTest := utils.NewIntegrationTest(testCases)
				jen.Id("runTest").Op(":=").Qual(utilsPkg, "NewTestSuite").Call(jen.Id("testCases")),
				// runTest(t)
				jen.Id("runTest").Call(jen.Id("t")),
			)

		if err := f.Save(out); err != nil {
			log.Fatalf("failed writing file: %v\n", err)
		}
	}
}

func testFile(packageName string) *jen.File {
	f := jen.NewFile(packageName)
	f.HeaderComment("Unless explicitly stated otherwise all files in this repository are licensed")
	f.HeaderComment("under the Apache License Version 2.0.")
	f.HeaderComment("This product includes software developed at Datadog (https://www.datadoghq.com/).")
	f.HeaderComment("Copyright 2023-present Datadog, Inc.")
	f.HeaderComment("")
	f.HeaderComment("Code generated by 'go generate'; DO NOT EDIT.")

	f.Comment("//go:build integration")

	return f
}

func parseCode(testDir string) (string, []string) {
	fset := token.NewFileSet()
	pkgs, err := parser.ParseDir(fset, testDir, nil, parser.ParseComments)
	if err != nil {
		log.Fatalf("failed to parse AST for dir: %v\n", err)
	}
	if len(pkgs) != 1 {
		log.Fatalf("%s: expected exactly 1 package, got %d", testDir, len(pkgs))
	}
	var testCases []string
	var pkgName string

	for name, pkg := range pkgs {
		pkgName = name
		for _, f := range pkg.Files {
			for _, decl := range f.Decls {
				gd, ok := decl.(*ast.GenDecl)
				if !ok || gd.Tok != token.TYPE {
					continue
				}
				for _, sp := range gd.Specs {
					typeSpec, ok := sp.(*ast.TypeSpec)
					if !ok {
						continue
					}
					name := typeSpec.Name.String()
					if strings.HasPrefix(name, "TestCase") {
						testCases = append(testCases, name)
					}
				}
			}
		}
	}
	// ensure order in test cases as well
	slices.Sort(testCases)
	return pkgName, testCases
}
