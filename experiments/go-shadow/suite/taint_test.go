// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package suite

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"testing"
)

const dirtyReport = "taint: os.Open received runtime shadow label\n"

type fixtureCase struct {
	Name         string            `json:"name"`
	TaintPath    string            `json:"taintPath"`
	DirtyReports int               `json:"dirtyReports"`
	TaintEnabled bool              `json:"taintEnabled"`
	Race         bool              `json:"race"`
	Environment  map[string]string `json:"env"`
	directory    string
}

func TestRuntimeTaintFixtures(t *testing.T) {
	goTool := os.Getenv("TAINT_GO")
	if goTool == "" {
		t.Skip("TAINT_GO must point to the patched Go binary")
	}
	goTool, err := filepath.Abs(goTool)
	if err != nil {
		t.Fatalf("resolve TAINT_GO: %v", err)
	}
	if _, err := os.Stat(goTool); err != nil {
		t.Fatalf("stat TAINT_GO: %v", err)
	}

	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve suite path")
	}
	fixtureRoot := filepath.Join(filepath.Dir(filename), "..", "fixture")

	cases, err := discoverFixtureCases(fixtureRoot)
	if err != nil {
		t.Fatal(err)
	}

	for _, testCase := range cases {
		t.Run(testCase.Name, func(t *testing.T) {
			arguments := []string{"run"}
			if testCase.Race {
				arguments = append(arguments, "-race")
			}
			if testCase.TaintEnabled {
				arguments = append(arguments, "-gcflags=all=-d=taint=1")
			}
			arguments = append(arguments, ".")

			command := exec.Command(goTool, arguments...)
			command.Dir = testCase.directory
			command.Env = replaceEnv(os.Environ(), "TAINT_PATH", testCase.TaintPath)
			for key, value := range testCase.Environment {
				command.Env = replaceEnv(command.Env, key, value)
			}
			output, err := command.CombinedOutput()
			if err != nil {
				t.Fatalf("run fixture: %v\n%s", err, output)
			}
			if strings.Contains(string(output), "IAST-SOURCE") || strings.Contains(string(output), "IAST-SINK") {
				t.Fatalf("temporary diagnostics leaked:\n%s", output)
			}
			if reports := strings.Count(string(output), dirtyReport); reports != testCase.DirtyReports {
				t.Fatalf("dirty reports = %d, want %d:\n%s", reports, testCase.DirtyReports, output)
			}
		})
	}
}

func discoverFixtureCases(fixtureRoot string) ([]fixtureCase, error) {
	manifestPaths, err := filepath.Glob(filepath.Join(fixtureRoot, "*", "cases.json"))
	if err != nil {
		return nil, fmt.Errorf("glob fixture manifests: %w", err)
	}
	mainPaths, err := filepath.Glob(filepath.Join(fixtureRoot, "*", "main.go"))
	if err != nil {
		return nil, fmt.Errorf("glob fixture programs: %w", err)
	}

	manifestDirectories := make(map[string]struct{}, len(manifestPaths))
	for _, manifestPath := range manifestPaths {
		manifestDirectories[filepath.Dir(manifestPath)] = struct{}{}
	}
	for _, mainPath := range mainPaths {
		directory := filepath.Dir(mainPath)
		if _, ok := manifestDirectories[directory]; !ok {
			return nil, fmt.Errorf("fixture %q has main.go but no cases.json", filepath.Base(directory))
		}
	}

	var cases []fixtureCase
	caseManifests := make(map[string]string)
	for _, manifestPath := range manifestPaths {
		contents, err := os.ReadFile(manifestPath)
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", manifestPath, err)
		}
		decoder := json.NewDecoder(strings.NewReader(string(contents)))
		decoder.DisallowUnknownFields()
		var manifestCases []fixtureCase
		if err := decoder.Decode(&manifestCases); err != nil {
			return nil, fmt.Errorf("decode %s: %w", manifestPath, err)
		}
		if err := decoder.Decode(&json.RawMessage{}); err != io.EOF {
			return nil, fmt.Errorf("decode %s: trailing JSON content", manifestPath)
		}

		for _, testCase := range manifestCases {
			if testCase.Name == "" {
				return nil, fmt.Errorf("decode %s: case name must not be empty", manifestPath)
			}
			if previousManifest, exists := caseManifests[testCase.Name]; exists {
				return nil, fmt.Errorf("duplicate case name %q in %s and %s", testCase.Name, previousManifest, manifestPath)
			}
			caseManifests[testCase.Name] = manifestPath
			testCase.directory = filepath.Dir(manifestPath)
			cases = append(cases, testCase)
		}
	}

	sort.Slice(cases, func(left, right int) bool {
		return cases[left].Name < cases[right].Name
	})
	return cases, nil
}

func replaceEnv(environment []string, key, value string) []string {
	prefix := key + "="
	result := make([]string, 0, len(environment)+1)
	for _, entry := range environment {
		if !strings.HasPrefix(entry, prefix) {
			result = append(result, entry)
		}
	}
	return append(result, prefix+value)
}
