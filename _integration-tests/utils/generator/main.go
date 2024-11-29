// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package main

import (
	_ "embed" // For go:embed
	"flag"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"log"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"
	"text/template"
)

const (
	genTestName = "gen_test.go"
	utilsPkg    = "datadoghq.dev/orchestrion/_integration-tests/utils"
)

var (
	//go:embed "testfile.tmpl"
	templateText string
	fileTemplate = template.Must(template.New("").Parse(templateText))
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
		testData := parseCode(testDir)
		if len(testData.Cases) == 0 {
			log.Fatalf("No test cases found in %s\n", testDir)
		}

		if err := testData.generate(filepath.Join(testDir, genTestName)); err != nil {
			log.Fatalln(err)
		}
	}
}

type (
	testCases struct {
		BuildConstraint string
		PkgName         string
		Cases           []testCase
	}
	testCase struct {
		TestName  string
		ClassName string
	}
)

func parseCode(testDir string) testCases {
	fset := token.NewFileSet()
	pkgs, err := parser.ParseDir(
		fset,
		testDir,
		func(fi fs.FileInfo) bool { return fi.Name() != genTestName },
		parser.ParseComments,
	)
	if err != nil {
		log.Fatalf("failed to parse AST for dir: %v\n", err)
	}
	if len(pkgs) != 1 {
		log.Fatalf("%s: expected exactly 1 package, got %d", testDir, len(pkgs))
	}

	var (
		pkgName string
		pkg     *ast.Package
	)
	for name, val := range pkgs {
		// NB -- There is exactly 1 item in the map
		pkgName = name
		pkg = val
	}

	var (
		buildConstraint = "integration"
		cases           []testCase
	)
	for _, f := range pkg.Files {
		if constraint := getBuildConstraint(f); len(constraint) > len(buildConstraint) {
			buildConstraint = constraint
		}

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
					testName := name[8:]
					cases = append(cases, testCase{TestName: testName, ClassName: name})
				}
			}
		}
	}

	// ensure order in test cases as well and remove repeated elements (e.g. in case of different OS implementations)
	slices.SortFunc(cases, func(lhs testCase, rhs testCase) int { return strings.Compare(lhs.TestName, rhs.TestName) })
	cases = slices.Compact(cases)

	return testCases{BuildConstraint: buildConstraint, PkgName: pkgName, Cases: cases}
}

func (t *testCases) generate(filename string) error {
	file, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer file.Close()

	tmpl, err := fileTemplate.Clone()
	if err != nil {
		return err
	}

	return tmpl.Execute(file, t)
}

func getBuildConstraint(f *ast.File) string {
	const defaultConstraint = "integration"

	pkgPos := f.Package
	for _, grp := range f.Comments {
		for _, cmt := range grp.List {
			if cmt.Slash > pkgPos {
				return defaultConstraint
			}

			if strings.HasPrefix(cmt.Text, "//go:build ") {
				return cmt.Text[11:]
			}
			if cmt.Text == "//generator:ignore-build-constraint" {
				return defaultConstraint
			}
		}
	}

	return defaultConstraint
}
