// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package injector_test

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/datadog/orchestrion/internal/injector"
	"github.com/datadog/orchestrion/internal/injector/aspect"
	"github.com/datadog/orchestrion/internal/injector/typed"
	"github.com/hexops/gotextdiff"
	"github.com/hexops/gotextdiff/myers"
	"github.com/hexops/gotextdiff/span"
	"github.com/sergi/go-diff/diffmatchpatch"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/tools/go/packages"
	"gopkg.in/yaml.v3"
	"gotest.tools/v3/golden"
)

type testConfig struct {
	Aspects             []aspect.Aspect                `yaml:"aspects"`
	PreserveLineInfo    bool                           `yaml:"preserveLineInfo"`
	SyntheticReferences map[string]typed.ReferenceKind `yaml:"syntheticReferences"`
	Code                string                         `yaml:"code"`
}

func Test(t *testing.T) {
	t.Parallel()

	differ := diffmatchpatch.New()
	differ.PatchMargin = 5

	_, thisFile, _, _ := runtime.Caller(0)
	dirName := "injector"
	testsDir := filepath.Join(thisFile, "..", "testdata", dirName)
	rootDir := filepath.Join(thisFile, "..", "..", "..")
	integDir := filepath.Join(rootDir, "_integration-tests")

	entries, err := os.ReadDir(testsDir)
	require.NoError(t, err, "failed to read test data directory")
	for _, item := range entries {
		if !item.IsDir() {
			continue
		}

		testName := item.Name()
		testPath := filepath.Join(testsDir, testName)
		t.Run(testName, func(t *testing.T) {
			t.Parallel()

			data, err := os.ReadFile(filepath.Join(testPath, "config.yml"))
			require.NoError(t, err, "failed to read test configuration")
			var config testConfig
			require.NoError(t, yaml.Unmarshal(data, &config), "failed to parse test configuration")

			tmp := t.TempDir()
			runGo(t, tmp, "mod", "init", "dummy/test/module")
			runGo(t, tmp, "mod", "edit", "-replace", fmt.Sprintf("github.com/datadog/orchestrion=%s", rootDir))
			runGo(t, tmp, "mod", "edit", "-replace", fmt.Sprintf("orchestrion/integration=%s", integDir))

			inputFile := filepath.Join(tmp, "input.go")
			original := strings.TrimSpace(config.Code) + "\n"
			require.NoError(t, os.WriteFile(inputFile, []byte(original), 0o644), "failed to create main.go")
			runGo(t, tmp, "mod", "tidy")

			inj := injector.Injector{
				Aspects:          config.Aspects,
				PreserveLineInfo: config.PreserveLineInfo,
				ModifiedFile:     func(path string) string { return filepath.Join(tmp, filepath.Base(path)+".edited.go") },
				ImportPath:       "dummy/test/module",
				LookupImport: func(path string) (io.ReadCloser, error) {
					pkgs, err := packages.Load(&packages.Config{Mode: packages.NeedExportFile, Dir: tmp}, path)
					if err != nil {
						return nil, err
					}
					file := pkgs[0].ExportFile
					if file == "" {
						return nil, fmt.Errorf("no export data for %s", path)
					}
					return os.Open(file)
				},
			}

			results, err := inj.InjectFiles([]string{inputFile})
			require.NoError(t, err, "failed to inject file")
			require.Len(t, results, 1, "expected exactly one result item")

			res := results[0]
			if res.Modified {
				assert.NotEqual(t, inputFile, res.Filename)
			} else {
				assert.Equal(t, inputFile, res.Filename)
			}

			modifiedData, err := os.ReadFile(res.Filename)
			require.NoError(t, err, "failed to read modified file")
			modified := normalize(modifiedData, inputFile)

			assert.Equal(t, config.SyntheticReferences, res.References.Map())

			edits := myers.ComputeEdits(span.URIFromPath("input.go"), original, modified)
			diff := gotextdiff.ToUnified("input.go", "output.go", original, edits)
			golden.Assert(t, fmt.Sprint(diff), filepath.Join(dirName, testName, "expected.diff"))

			if res.Modified {
				// Verify that the modified code compiles...
				os.Rename(res.Filename, inputFile)
				runGo(t, tmp, "mod", "tidy")
				runGo(t, tmp, "build", inputFile)
			}
		})
	}
}

func runGo(t *testing.T, dir string, args ...string) {
	t.Helper()

	cmd := exec.Command("go", args...)
	cmd.Dir = dir
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	require.NoError(t, cmd.Run(), "failed running go %s", strings.Join(args, " "))
}

// normalize replaces all tabulation characters with two spaces, matching the indentation style found in YAML documents,
// and cleans up line directives to remove the temporary file name.
func normalize(in []byte, filename string) string {
	res := strings.ReplaceAll(string(in), "\t", "  ")
	res = strings.ReplaceAll(res, fmt.Sprintf("//line %s:", filename), "//line input.go:")
	return res
}
