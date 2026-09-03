// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package instrument

import (
	"bytes"
	"context"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"
)

type expectedCase struct {
	id          int
	name        string
	wantsReport bool
}

type caseResult struct {
	passed     bool
	diagnostic string
}

type e2eModule struct {
	root      string
	directory string
	expected  []expectedCase
}

func Test_InstrumentedProgramReportsTaintedOpenPath_when_SourceCrossesLanguageOperations(t *testing.T) {
	// Given
	fixture := prepareE2EModule(t, true)
	orchestrion := buildOrchestrion(t, fixture.root)
	program := filepath.Join(t.TempDir(), "iast-e2e")
	runCommand(t, fixture.directory, nil, 2*time.Minute, orchestrion, "go", "build", "-p=1", "-o", program, ".")

	// When
	output := runCommand(t, fixture.directory, nil, 5*time.Second, program)

	// Then
	assertCaseResults(t, fixture.expected, output)
}

func Test_ProgramEmitsNoTaintReports_when_InstrumentationIsDisabled(t *testing.T) {
	for _, test := range []struct {
		name        string
		includeTool bool
	}{
		{name: "tracking disabled", includeTool: false},
		{name: "integration configured but disabled", includeTool: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			// Given
			fixture := prepareE2EModule(t, test.includeTool)
			program := filepath.Join(t.TempDir(), "iast-e2e")
			runCommand(t, fixture.directory, nil, 2*time.Minute, "go", "build", "-p=1", "-o", program, ".")

			// When
			output := runCommand(t, fixture.directory, nil, 30*time.Second, program)

			// Then
			assertNoTaintReports(t, fixture.expected, output)
		})
	}
}

func Test_InstrumentedProgramReportsExpectedTaint_when_BuiltWithRaceDetector(t *testing.T) {
	// Given
	fixture := prepareE2EModule(t, true)
	orchestrion := buildOrchestrion(t, fixture.root)
	program := filepath.Join(t.TempDir(), "iast-e2e-race")
	runCommand(t, fixture.directory, nil, 5*time.Minute, orchestrion, "go", "build", "-race", "-p=1", "-o", program, ".")

	// When
	output := runCommand(t, fixture.directory, nil, 30*time.Second, program)

	// Then
	assertCaseResults(t, fixture.expected, output)
}

func Test_InstrumentedProgramPreservesExactRangePast65536(t *testing.T) {
	// Given
	root := repositoryRoot(t)
	orchestrion := buildOrchestrion(t, root)
	module := t.TempDir()
	goMod := "module example.com/taint-capacity\n\ngo 1.25.0\n\nrequire github.com/DataDog/orchestrion v0.0.0\n\nreplace github.com/DataDog/orchestrion => " + root + "\n"
	if err := os.WriteFile(filepath.Join(module, "go.mod"), []byte(goMod), 0o644); err != nil {
		t.Fatalf("write go.mod: %v", err)
	}
	tool := `//go:build tools

package tools

import _ "github.com/DataDog/orchestrion/runtime/taint/instrument"
`
	if err := os.WriteFile(filepath.Join(module, "orchestrion.tool.go"), []byte(tool), 0o644); err != nil {
		t.Fatalf("write orchestrion tool file: %v", err)
	}
	programSource := `package main

import (
	"fmt"
	"os"
	"runtime"

	"github.com/DataDog/orchestrion/runtime/taint"
)

func main() {
	const environment = "IAST_CAPACITY_INPUT"
	const value = "capacity-secret"
	if err := os.Setenv(environment, value); err != nil {
		panic(err)
	}

	values := make([]string, 0, 1<<16)
	for range 1 << 16 {
		values = append(values, os.Getenv(environment))
	}
	dirty := os.Getenv(environment)
	reports := make([]taint.Report, 0, 1)
	restore := taint.SetReporter(func(report taint.Report) {
		reports = append(reports, report)
	})
	_, _ = os.Open(dirty)
	_, _ = os.Open(value)
	runtime.KeepAlive(values)
	restore()

	if len(reports) != 1 {
		panic("unexpected report count")
	}
	report := reports[0]
	if report.Sink != "os.Open" || report.Value != value || len(report.Ranges) != 1 || report.Ranges[0].Start != 0 || report.Ranges[0].End != len(value) {
		panic("unexpected report")
	}
	fmt.Println("PASS")
}
`
	if err := os.WriteFile(filepath.Join(module, "main.go"), []byte(programSource), 0o644); err != nil {
		t.Fatalf("write main.go: %v", err)
	}
	runCommand(t, module, nil, 2*time.Minute, "go", "mod", "tidy")

	for _, test := range []struct {
		name string
		args []string
	}{
		{name: "normal", args: []string{"go", "build", "-p=1"}},
		{name: "race", args: []string{"go", "build", "-race", "-p=1"}},
	} {
		t.Run(test.name, func(t *testing.T) {
			program := filepath.Join(t.TempDir(), "iast-capacity")
			arguments := append(test.args, "-o", program, ".")
			runCommand(t, module, nil, 5*time.Minute, orchestrion, arguments...)

			// When
			output := runCommand(t, module, nil, time.Minute, program)

			// Then
			if output != "PASS\n" {
				t.Fatalf("program output = %q, want %q", output, "PASS\n")
			}
		})
	}
}

func discoverCases(t *testing.T, directory string) []expectedCase {
	t.Helper()
	files, err := filepath.Glob(filepath.Join(directory, "case_*.go"))
	if err != nil {
		t.Fatalf("discover case files: %v", err)
	}
	if len(files) == 0 {
		t.Fatal("no E2E case files found")
	}

	cases := make([]expectedCase, 0, len(files))
	seenNames := make(map[string]string, len(files))
	for _, file := range files {
		current := parseCaseRegistration(t, file)
		if previous, duplicate := seenNames[current.name]; duplicate {
			t.Fatalf("duplicate case name %q in %s and %s", current.name, previous, file)
		}
		seenNames[current.name] = file
		cases = append(cases, current)
	}
	return cases
}

func prepareE2EModule(t *testing.T, includeTool bool) e2eModule {
	t.Helper()
	root := repositoryRoot(t)
	fixtures := filepath.Join(root, "runtime", "taint", "instrument", "testdata", "e2e")
	module := t.TempDir()
	copyFixtures(t, fixtures, module)
	if !includeTool {
		if err := os.Remove(filepath.Join(module, "orchestrion.tool.go")); err != nil {
			t.Fatalf("remove Orchestrion tool fixture: %v", err)
		}
	}
	materializeGoMod(t, module, root)
	runCommand(t, module, nil, 2*time.Minute, "go", "mod", "tidy")
	return e2eModule{root: root, directory: module, expected: discoverCases(t, fixtures)}
}

func buildOrchestrion(t *testing.T, root string) string {
	t.Helper()
	orchestrion := filepath.Join(t.TempDir(), "orchestrion")
	runCommand(t, root, nil, 2*time.Minute, "go", "build", "-o", orchestrion, ".")
	return orchestrion
}

func parseCaseRegistration(t *testing.T, file string) expectedCase {
	t.Helper()
	parsed, err := parser.ParseFile(token.NewFileSet(), file, nil, 0)
	if err != nil {
		t.Fatalf("parse %s: %v", file, err)
	}
	var registrations []*ast.CallExpr
	ast.Inspect(parsed, func(node ast.Node) bool {
		call, ok := node.(*ast.CallExpr)
		if !ok {
			return true
		}
		identifier, ok := call.Fun.(*ast.Ident)
		if ok && identifier.Name == "register" {
			registrations = append(registrations, call)
		}
		return true
	})
	if len(registrations) != 1 || len(registrations[0].Args) != 1 {
		t.Fatalf("%s must contain exactly one register(Case{...}) call", file)
	}
	literal, ok := registrations[0].Args[0].(*ast.CompositeLit)
	if !ok {
		t.Fatalf("%s registration argument must be a Case literal", file)
	}

	current := expectedCase{id: -1}
	for _, element := range literal.Elts {
		field, ok := element.(*ast.KeyValueExpr)
		if !ok {
			continue
		}
		key, ok := field.Key.(*ast.Ident)
		if !ok {
			continue
		}
		switch key.Name {
		case "ID":
			value, ok := field.Value.(*ast.BasicLit)
			if !ok {
				continue
			}
			current.id, err = strconv.Atoi(value.Value)
			if err != nil {
				t.Fatalf("parse ID in %s: %v", file, err)
			}
		case "Name":
			value, ok := field.Value.(*ast.BasicLit)
			if !ok {
				continue
			}
			current.name, err = strconv.Unquote(value.Value)
			if err != nil {
				t.Fatalf("parse Name in %s: %v", file, err)
			}
		case "Want":
			value, ok := field.Value.(*ast.CompositeLit)
			if !ok {
				t.Fatalf("parse Want in %s: expected composite literal", file)
			}
			current.wantsReport = len(value.Elts) > 0
		}
	}
	if current.id < 0 || current.name == "" {
		t.Fatalf("%s Case literal must have static ID and Name fields", file)
	}
	return current
}

func copyFixtures(t *testing.T, source, destination string) {
	t.Helper()
	entries, err := os.ReadDir(source)
	if err != nil {
		t.Fatalf("read fixtures: %v", err)
	}
	for _, entry := range entries {
		if entry.IsDir() {
			t.Fatalf("unexpected fixture directory %s", entry.Name())
		}
		contents, err := os.ReadFile(filepath.Join(source, entry.Name()))
		if err != nil {
			t.Fatalf("read fixture %s: %v", entry.Name(), err)
		}
		if err := os.WriteFile(filepath.Join(destination, entry.Name()), contents, 0o644); err != nil {
			t.Fatalf("write fixture %s: %v", entry.Name(), err)
		}
	}
}

func materializeGoMod(t *testing.T, module, root string) {
	t.Helper()
	template, err := os.ReadFile(filepath.Join(module, "go.mod.txt"))
	if err != nil {
		t.Fatalf("read go.mod template: %v", err)
	}
	contents := strings.ReplaceAll(string(template), "__REPO_ROOT__", root)
	if err := os.WriteFile(filepath.Join(module, "go.mod"), []byte(contents), 0o644); err != nil {
		t.Fatalf("write go.mod: %v", err)
	}
}

func parseCaseResults(t *testing.T, output string) map[expectedCase]caseResult {
	t.Helper()
	if strings.Contains(output, "IAST-SOURCE") || strings.Contains(output, "IAST-SINK") {
		t.Fatalf("unexpected IAST debug marker in output:\n%s", output)
	}
	results := make(map[expectedCase]caseResult)
	for line := range strings.SplitSeq(strings.TrimSpace(output), "\n") {
		parts := strings.SplitN(line, "\t", 5)
		if len(parts) < 4 || parts[0] != "CASE" {
			t.Fatalf("unexpected program output line %q", line)
		}
		id, err := strconv.Atoi(parts[1])
		if err != nil {
			t.Fatalf("parse case ID in %q: %v", line, err)
		}
		current := expectedCase{id: id, name: parts[2]}
		if _, duplicate := results[current]; duplicate {
			t.Fatalf("duplicate result for case %03d %q", current.id, current.name)
		}
		switch parts[3] {
		case "PASS":
			if len(parts) != 4 {
				t.Fatalf("malformed PASS line %q", line)
			}
			results[current] = caseResult{passed: true}
		case "FAIL":
			if len(parts) != 5 || parts[4] == "" {
				t.Fatalf("malformed FAIL line %q", line)
			}
			results[current] = caseResult{diagnostic: parts[4]}
		default:
			t.Fatalf("unknown case status in %q", line)
		}
	}
	return results
}

func assertCaseResults(t *testing.T, expected []expectedCase, output string) {
	t.Helper()
	results := parseCaseResults(t, output)
	for _, current := range expected {
		key := expectedCase{id: current.id, name: current.name}
		result, found := results[key]
		if !found {
			t.Errorf("case %03d %q produced no result", current.id, current.name)
			continue
		}
		delete(results, key)
		t.Run(current.name, func(t *testing.T) {
			if !result.passed {
				t.Fatal(result.diagnostic)
			}
		})
	}
	for current := range results {
		t.Errorf("result for unregistered case %03d %q", current.id, current.name)
	}
}

func assertNoTaintReports(t *testing.T, expected []expectedCase, output string) {
	t.Helper()
	results := parseCaseResults(t, output)
	for _, current := range expected {
		key := expectedCase{id: current.id, name: current.name}
		result, found := results[key]
		if !found {
			t.Errorf("case %03d %q produced no result", current.id, current.name)
			continue
		}
		delete(results, key)
		if current.wantsReport {
			if result.passed || !strings.Contains(result.diagnostic, "captured=[]") {
				t.Errorf("case %03d %q captured a taint report: %s", current.id, current.name, result.diagnostic)
			}
			continue
		}
		if !result.passed {
			t.Errorf("case %03d %q failed without instrumentation: %s", current.id, current.name, result.diagnostic)
		}
	}
	for current := range results {
		t.Errorf("result for unregistered case %03d %q", current.id, current.name)
	}
}

func repositoryRoot(t *testing.T) string {
	t.Helper()
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller() failed")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(filename), "..", "..", ".."))
}

func runCommand(t *testing.T, directory string, environment []string, timeout time.Duration, name string, arguments ...string) string {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	command := exec.CommandContext(ctx, name, arguments...)
	command.Dir = directory
	command.Env = append(os.Environ(), environment...)
	var output bytes.Buffer
	command.Stdout = &output
	command.Stderr = &output
	if err := command.Run(); err != nil {
		if ctx.Err() != nil {
			t.Fatalf("%s %s: %v after %s\n%s", name, strings.Join(arguments, " "), ctx.Err(), timeout, output.String())
		}
		t.Fatalf("%s %s: %v\n%s", name, strings.Join(arguments, " "), err, output.String())
	}
	return output.String()
}
