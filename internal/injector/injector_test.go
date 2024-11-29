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

	"github.com/DataDog/orchestrion/internal/injector"
	"github.com/DataDog/orchestrion/internal/injector/aspect"
	"github.com/DataDog/orchestrion/internal/injector/aspect/context"
	"github.com/DataDog/orchestrion/internal/injector/typed"
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
	Aspects             []*aspect.Aspect               `yaml:"aspects"`
	SyntheticReferences map[string]typed.ReferenceKind `yaml:"syntheticReferences"`
	GoLang              context.GoLangVersion          `yaml:"required-lang"`
	Code                string                         `yaml:"code"`
}

const testModuleName = "dummy/test/module"

func Test(t *testing.T) {
	t.Parallel()

	differ := diffmatchpatch.New()
	differ.PatchMargin = 5

	_, thisFile, _, _ := runtime.Caller(0)
	dirName := "injector"
	testsDir := filepath.Join(thisFile, "..", "testdata", dirName)
	rootDir := filepath.Join(thisFile, "..", "..", "..")

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

			tmp := t.TempDir()

			testLookup := func(path string) (io.ReadCloser, error) {
				pkgs, err := packages.Load(
					&packages.Config{
						Mode: packages.NeedExportFile,
						Dir:  tmp,
						Logf: t.Logf,
					},
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

			data, err := os.ReadFile(filepath.Join(testPath, "config.yml"))
			require.NoError(t, err, "failed to read test configuration")
			var config testConfig
			require.NoError(t, yaml.Unmarshal(data, &config), "failed to parse test configuration")

			runGo(t, tmp, "mod", "init", testModuleName)
			runGo(t, tmp, "mod", "edit", "-replace", fmt.Sprintf("github.com/DataDog/orchestrion=%s", rootDir))

			inputFile := filepath.Join(tmp, "input.go")
			original := strings.TrimSpace(config.Code) + "\n"
			require.NoError(t, os.WriteFile(inputFile, []byte(original), 0o644), "failed to create main.go")
			runGo(t, tmp, "mod", "tidy")

			inj := injector.Injector{
				Aspects:      config.Aspects,
				ModifiedFile: func(path string) string { return filepath.Join(tmp, filepath.Base(path)+".edited.go") },
				ImportPath:   testModuleName,
				Lookup:       testLookup,
			}

			res, resGoLang, err := inj.InjectFiles([]string{inputFile})
			require.NoError(t, err, "failed to inject file")

			resFile, modified := res[inputFile]
			if !modified {
				golden.Assert(t, "", filepath.Join(dirName, testName, "expected.diff"))
				return
			}

			assert.Equal(t, filepath.Join(tmp, filepath.Base(inputFile)+".edited.go"), resFile.Filename)
			assert.Equal(t, config.SyntheticReferences, resFile.References.Map())
			assert.Equal(t, config.GoLang.String(), resGoLang.String())

			modifiedData, err := os.ReadFile(resFile.Filename)
			require.NoError(t, err, "failed to read modified file")
			normalized := normalize(modifiedData, inputFile)

			edits := myers.ComputeEdits(span.URIFromPath("input.go"), original, normalized)
			diff := gotextdiff.ToUnified("input.go", "output.go", original, edits)
			golden.Assert(t, fmt.Sprint(diff), filepath.Join(dirName, testName, "expected.diff"))

			// Verify that the modified code compiles...
			os.Rename(resFile.Filename, inputFile)
			runGo(t, tmp, "mod", "tidy")
			runGo(t, tmp, "build", inputFile)
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
