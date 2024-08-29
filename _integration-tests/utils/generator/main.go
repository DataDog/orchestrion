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

func init() {
	flag.Usage = func() {
		_, _ = fmt.Fprintf(os.Stderr, "Usage: %s <dir>\n", os.Args[0])
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

	file := jen.NewFile("tests")
	file.HeaderComment("Unless explicitly stated otherwise all files in this repository are licensed")
	file.HeaderComment("under the Apache License Version 2.0.")
	file.HeaderComment("This product includes software developed at Datadog (https://www.datadoghq.com/).")
	file.HeaderComment("Copyright 2023-present Datadog, Inc.")
	file.HeaderComment("")
	file.HeaderComment("Code generated by 'go generate'; DO NOT EDIT.")

	file.Comment("//go:build integration")

	entries, err := os.ReadDir(root)
	if err != nil {
		log.Fatalf("failed listing directory: %v\n", err)
	}
	// Ensure stable ordering by explicitly sorting...
	slices.SortFunc(entries, func(lhs, rhs os.DirEntry) int {
		return strings.Compare(lhs.Name(), rhs.Name())
	})

	fset := token.NewFileSet()

	file.Var().Id("suite").Op("=").Map(jen.String()).Id("testCase").ValuesFunc(func(g *jen.Group) {
		for _, entry := range entries {
			if !entry.IsDir() {
				continue
			}

			p := path.Join(root, entry.Name())

			pkgs, err := parser.ParseDir(fset, p, nil, parser.ParseComments)
			if err != nil {
				log.Fatalf("failed to parse AST for dir: %v\n", err)
			}
			if len(pkgs) != 1 {
				log.Fatalf("%s: expected exactly 1 package, got %d", p, len(pkgs))
			}
			var testCases []string
			for _, pkg := range pkgs {
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

			for _, tc := range testCases {
				tcName := entry.Name()
				if n := strings.TrimPrefix(tc, "TestCase"); n != "" {
					tcName = fmt.Sprintf("%s/%s", entry.Name(), n)
				}
				g.Line().Lit(tcName).Op(":").New(jen.Qual(fmt.Sprintf("orchestrion/integration/tests/%s", entry.Name()), tc))
			}
		}
		g.Line().Empty()
	})

	if err := file.Save("suite.generated.go"); err != nil {
		log.Fatalf("failed writing file: %v\n", err)
	}
}
