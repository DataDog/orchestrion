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
	"reflect"
	"runtime"
	"sort"
	"strings"
	"testing"
)

const dirtyReport = "taint: os.Open received runtime shadow label\n"

// requiredCompilerMarker is the identity the patched compiler appends to its own
// -V=full output. TestRuntimeTaintFixtures refuses to produce evidence unless TAINT_GO
// reports it, because a stock or differently-patched toolchain can otherwise emit output
// that satisfies these assertions and be mistaken for a green Lane B run.
const requiredCompilerMarker = "iast-taint-shadow-v28"

type fixtureCase struct {
	Name                        string            `json:"name"`
	TaintPath                   string            `json:"taintPath"`
	DirtyReports                int               `json:"dirtyReports"`
	TaintEnabled                bool              `json:"taintEnabled"`
	Race                        bool              `json:"race"`
	Environment                 map[string]string `json:"env"`
	UninstrumentedPackages      []string          `json:"uninstrumentedPackages"`
	ExactRanges                 [][2]int          `json:"exactRanges"`
	ExactSourceRangeCoordinates [][2]int          `json:"exactSourceRangeCoordinates"`
	DistinctSourceIDs           int               `json:"distinctSourceIDs"`
	directory                   string
}

// TestRuntimeTaintFixtures is the Lane B evidence gate. It only runs when TAINT_GO points
// at the patched toolchain, so a plain `go test ./...` skips it - see TestFixtureInventory,
// which validates the manifests on every run and therefore still fails on fixture rot.
func TestRuntimeTaintFixtures(t *testing.T) {
	goTool := os.Getenv("TAINT_GO")
	if goTool == "" {
		t.Skip("TAINT_GO is unset: 0 of the Lane B fixture cases ran. This is NOT evidence; " +
			"set TAINT_GO to a toolchain reporting " + requiredCompilerMarker + " to produce it")
	}
	goTool, err := filepath.Abs(goTool)
	if err != nil {
		t.Fatalf("resolve TAINT_GO: %v", err)
	}
	if _, err := os.Stat(goTool); err != nil {
		t.Fatalf("stat TAINT_GO: %v", err)
	}
	requirePatchedCompiler(t, goTool)

	fixtureRoot := locateFixtureRoot(t)

	cases, err := discoverFixtureCases(fixtureRoot)
	if err != nil {
		t.Fatal(err)
	}
	if len(cases) == 0 {
		t.Fatal("discovered 0 fixture cases; an empty suite must never report success")
	}

	for _, testCase := range cases {
		t.Run(testCase.Name, func(t *testing.T) {
			arguments := []string{"run"}
			if testCase.Race {
				arguments = append(arguments, "-race")
			}
			if testCase.TaintEnabled {
				arguments = append(arguments, "-gcflags=all=-d=taint=1")
				for _, packagePath := range testCase.UninstrumentedPackages {
					arguments = append(arguments, "-gcflags="+packagePath+"=-d=taint=0")
				}
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
			if len(testCase.ExactRanges) != 0 {
				const exactRangesPrefix = "taint: os.Open ranges="
				var actual [][2]int
				reports := 0
				for line := range strings.SplitSeq(string(output), "\n") {
					if !strings.HasPrefix(line, exactRangesPrefix) {
						continue
					}
					reports++
					if err := json.Unmarshal([]byte(strings.TrimPrefix(line, exactRangesPrefix)), &actual); err != nil {
						t.Fatalf("decode exact ranges: %v\n%s", err, output)
					}
				}
				if reports != 1 {
					t.Fatalf("exact range reports = %d, want 1:\n%s", reports, output)
				}
				if !reflect.DeepEqual(actual, testCase.ExactRanges) {
					t.Fatalf("exact ranges = %v, want %v:\n%s", actual, testCase.ExactRanges, output)
				}
			}
			if testCase.DistinctSourceIDs > 0 {
				const sourceRangesPrefix = "taint: os.Open sourceRanges="
				var actual [][3]uint64
				reports := 0
				for line := range strings.SplitSeq(string(output), "\n") {
					if !strings.HasPrefix(line, sourceRangesPrefix) {
						continue
					}
					reports++
					if err := json.Unmarshal([]byte(strings.TrimPrefix(line, sourceRangesPrefix)), &actual); err != nil {
						t.Fatalf("decode exact source ranges: %v\n%s", err, output)
					}
				}
				if reports != 1 {
					t.Fatalf("exact source range reports = %d, want 1:\n%s", reports, output)
				}
				coordinates := make([][2]int, len(actual))
				ids := make(map[uint64]struct{}, len(actual))
				for index, current := range actual {
					coordinates[index] = [2]int{int(current[0]), int(current[1])}
					if current[2] == 0 {
						t.Fatalf("exact source range has anonymous ID:\n%s", output)
					}
					ids[current[2]] = struct{}{}
				}
				if !reflect.DeepEqual(coordinates, testCase.ExactSourceRangeCoordinates) {
					t.Fatalf("exact source range coordinates = %v, want %v:\n%s", coordinates, testCase.ExactSourceRangeCoordinates, output)
				}
				if len(ids) != testCase.DistinctSourceIDs {
					t.Fatalf("distinct source IDs = %d, want %d:\n%s", len(ids), testCase.DistinctSourceIDs, output)
				}
			}
		})
	}
}

// TestFixtureInventory runs with the stock toolchain, so it is the only Lane B check that
// executes under a plain `go test ./...`. It exists because every fixture-level guarantee
// the ledger relies on - that no fixture is silently dropped, that no case defaults its
// expected report count, and that no case rewrites the source the harness set - is
// invisible to a run that skips TestRuntimeTaintFixtures.
func TestFixtureInventory(t *testing.T) {
	fixtureRoot := locateFixtureRoot(t)

	directories, err := filepath.Glob(filepath.Join(fixtureRoot, "*", "cases.json"))
	if err != nil {
		t.Fatalf("glob fixture manifests: %v", err)
	}
	if len(directories) == 0 {
		t.Fatalf("no fixture manifests under %s", fixtureRoot)
	}

	cases, err := discoverFixtureCases(fixtureRoot)
	if err != nil {
		t.Fatalf("discover fixture cases: %v", err)
	}

	contributing := make(map[string]struct{}, len(directories))
	for _, testCase := range cases {
		contributing[testCase.directory] = struct{}{}
	}

	// Every directory that ships a manifest must contribute at least one case. An empty
	// `[]` manifest keeps its main.go on disk while removing it from the suite, which
	// reads as coverage in a directory listing and is absent from the run.
	for _, manifestPath := range directories {
		directory := filepath.Dir(manifestPath)
		if _, ok := contributing[directory]; !ok {
			t.Errorf("fixture %q ships a manifest and a program but contributes 0 cases: an empty manifest silently removes it from the suite",
				filepath.Base(directory))
		}
	}

	t.Logf("fixture inventory: %d directories, %d cases", len(directories), len(cases))
}

func locateFixtureRoot(t *testing.T) string {
	t.Helper()
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve suite path")
	}
	return filepath.Join(filepath.Dir(filename), "..", "fixture")
}

// requirePatchedCompiler fails unless TAINT_GO's compiler self-identifies as the patched
// build the ledger cites. Without this, `go test` against a stock toolchain silently
// produces zero reports and every zero-report fixture passes.
func requirePatchedCompiler(t *testing.T, goTool string) {
	t.Helper()
	output, err := exec.Command(goTool, "tool", "compile", "-V=full").CombinedOutput()
	if err != nil {
		t.Fatalf("run %s tool compile -V=full: %v\n%s", goTool, err, output)
	}
	if version := strings.TrimSpace(string(output)); !strings.Contains(version, requiredCompilerMarker) {
		t.Fatalf("TAINT_GO compiler reports %q, want a build containing %q; refusing to record Lane B evidence from an unidentified toolchain",
			version, requiredCompilerMarker)
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
		if len(manifestCases) == 0 {
			return nil, fmt.Errorf("decode %s: manifest declares no cases; an empty manifest removes the fixture from the suite while leaving its program on disk", manifestPath)
		}

		// Decode a second time as raw objects so required keys can be distinguished from
		// keys that merely decoded to their zero value. A positive case that omits
		// dirtyReports would otherwise silently become a zero-report negative case.
		var rawCases []map[string]json.RawMessage
		if err := json.Unmarshal(contents, &rawCases); err != nil {
			return nil, fmt.Errorf("decode %s as raw objects: %w", manifestPath, err)
		}

		for index, testCase := range manifestCases {
			if testCase.Name == "" {
				return nil, fmt.Errorf("decode %s: case name must not be empty", manifestPath)
			}
			if _, declared := rawCases[index]["dirtyReports"]; !declared {
				return nil, fmt.Errorf("decode %s: case %q must declare dirtyReports explicitly, even when it is 0", manifestPath, testCase.Name)
			}
			if _, overridden := testCase.Environment["TAINT_PATH"]; overridden {
				return nil, fmt.Errorf("decode %s: case %q sets TAINT_PATH through env, which silently replaces the taintPath the harness applied; declare the real source in taintPath instead", manifestPath, testCase.Name)
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
