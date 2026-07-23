// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package suite

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

const dirtyReport = "taint: os.Open received runtime shadow label\n"

type fixtureCase struct {
	name         string
	fixture      string
	taintPath    string
	dirtyReports int
	taintEnabled bool
	raceEnabled  bool
	environment  map[string]string
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

	cases := []fixtureCase{
		{name: "disabled", fixture: "localcopy", taintPath: "/tmp/iast-disabled"},
		{name: "clean", fixture: "localcopy", taintEnabled: true},
		{name: "local copy", fixture: "localcopy", taintPath: "/tmp/iast-local", dirtyReports: 1, taintEnabled: true},
		{name: "phi", fixture: "phi", taintPath: "/tmp/iast-phi", dirtyReports: 1, taintEnabled: true, environment: map[string]string{"USE_DIRTY": "1"}},
		{name: "static call", fixture: "call", taintPath: "/tmp/iast-call", dirtyReports: 1, taintEnabled: true},
		{name: "function value", fixture: "functionvalue", taintPath: "/tmp/iast-function", dirtyReports: 1, taintEnabled: true},
		{name: "dynamic recursion", fixture: "dynamicrecursion", taintPath: "/tmp/iast-recursion", dirtyReports: 1, taintEnabled: true},
		{name: "static recursion", fixture: "recursion", taintPath: "/tmp/iast-static-recursion", dirtyReports: 1, taintEnabled: true},
		{name: "dynamic defer", fixture: "dynamicdefer", taintPath: "/tmp/iast-defer", dirtyReports: 1, taintEnabled: true},
		{name: "panic cleanup", fixture: "panic", taintPath: "/tmp/iast-panic", dirtyReports: 1, taintEnabled: true},
		{name: "buffered channel", fixture: "channel", taintPath: "/tmp/iast-buffered", dirtyReports: 1, taintEnabled: true},
		{name: "unbuffered channel", fixture: "channelunbuffered", taintPath: "/tmp/iast-unbuffered", dirtyReports: 1, taintEnabled: true},
		{name: "buffered channel GC", fixture: "channelgc", taintPath: "/tmp/iast-channel-gc", dirtyReports: 1, taintEnabled: true},
		{name: "select matrix", fixture: "selectcases", taintPath: "/tmp/iast-select", dirtyReports: 3, taintEnabled: true},
		{name: "select clean", fixture: "selectcases", taintEnabled: true},
		{name: "map assignment", fixture: "mapassign", taintPath: "/tmp/iast-map", dirtyReports: 1, taintEnabled: true},
		{name: "map literal", fixture: "mapvalue", taintPath: "/tmp/iast-map-literal", dirtyReports: 1, taintEnabled: true},
		{name: "map growth", fixture: "mapgrowth", taintPath: "/tmp/iast-map-growth", dirtyReports: 1, taintEnabled: true},
		{name: "map lifecycle", fixture: "mapcases", taintPath: "/tmp/iast-map-cases", dirtyReports: 4, taintEnabled: true},
		{name: "generic small map", fixture: "mapgenericsmall", taintPath: "/tmp/iast-map-generic", dirtyReports: 1, taintEnabled: true},
		{name: "closure environment", fixture: "closurevalue", taintPath: "/tmp/iast-closure", dirtyReports: 1, taintEnabled: true},
		{name: "heap address reuse", fixture: "heapreuse", taintPath: "/tmp/iast-reuse", dirtyReports: 1, taintEnabled: true},
		{name: "address-taken parameter", fixture: "addressparam", taintPath: "/tmp/iast-param", dirtyReports: 1, taintEnabled: true},
		{name: "unnamed parameters", fixture: "unnamedparam", taintPath: "/tmp/iast-unnamed", taintEnabled: true},
		{name: "interface dispatch", fixture: "interfacecall", taintPath: "/tmp/iast-interface", dirtyReports: 1, taintEnabled: true},
		{name: "interface receiver isolation", fixture: "interfacereceiver", taintPath: "/tmp/iast-receiver", taintEnabled: true},
		{name: "recovered return", fixture: "recoverreturn", taintPath: "/tmp/iast-recover", dirtyReports: 1, taintEnabled: true},
		{name: "zero-sized channel", fixture: "channelzero", taintEnabled: true},
		{name: "select race", fixture: "selectcases", taintPath: "/tmp/iast-select-race", dirtyReports: 3, taintEnabled: true, raceEnabled: true},
		{name: "map race", fixture: "mapcases", taintPath: "/tmp/iast-map-race", dirtyReports: 4, taintEnabled: true, raceEnabled: true},
		{name: "heap reuse race", fixture: "heapreuse", taintPath: "/tmp/iast-reuse-race", dirtyReports: 1, taintEnabled: true, raceEnabled: true},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			arguments := []string{"run"}
			if testCase.raceEnabled {
				arguments = append(arguments, "-race")
			}
			if testCase.taintEnabled {
				arguments = append(arguments, "-gcflags=all=-d=taint=1")
			}
			arguments = append(arguments, ".")

			command := exec.Command(goTool, arguments...)
			command.Dir = filepath.Join(fixtureRoot, testCase.fixture)
			command.Env = replaceEnv(os.Environ(), "TAINT_PATH", testCase.taintPath)
			for key, value := range testCase.environment {
				command.Env = replaceEnv(command.Env, key, value)
			}
			output, err := command.CombinedOutput()
			if err != nil {
				t.Fatalf("run fixture: %v\n%s", err, output)
			}
			if strings.Contains(string(output), "IAST-SOURCE") || strings.Contains(string(output), "IAST-SINK") {
				t.Fatalf("temporary diagnostics leaked:\n%s", output)
			}
			if reports := strings.Count(string(output), dirtyReport); reports != testCase.dirtyReports {
				t.Fatalf("dirty reports = %d, want %d:\n%s", reports, testCase.dirtyReports, output)
			}
		})
	}
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
