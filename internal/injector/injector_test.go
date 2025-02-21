// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package injector_test

import (
	gocontext "context"
	"fmt"
	"go/parser"
	"go/token"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/tools/go/packages"
	"gopkg.in/yaml.v3"
	"gotest.tools/v3/golden"

	"github.com/DataDog/orchestrion/internal/injector"
	"github.com/DataDog/orchestrion/internal/injector/aspect"
	"github.com/DataDog/orchestrion/internal/injector/aspect/context"
	"github.com/DataDog/orchestrion/internal/injector/typed"
)

type testConfig struct {
	Aspects             []*aspect.Aspect               `yaml:"aspects"`
	SyntheticReferences map[string]typed.ReferenceKind `yaml:"syntheticReferences"`
	GoLang              context.GoLangVersion          `yaml:"required-lang"`
	Code                string                         `yaml:"code"`
	ImportPath          string                         `yaml:"import-path"`
}

const testModuleName = "dummy/test/module"

func Test(t *testing.T) {
	t.Parallel()

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
					&packages.Config{Mode: packages.NeedExportFile, Dir: tmp},
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
			runGo(t, tmp, "mod", "edit",
				fmt.Sprintf("-replace=github.com/DataDog/orchestrion=%s", rootDir),
				fmt.Sprintf("-replace=github.com/DataDog/orchestrion/instrument=%s", filepath.Join(rootDir, "instrument")),
			)

			inputFile := filepath.Join(tmp, "input.go")
			original := strings.TrimSpace(config.Code) + "\n"
			require.NoError(t, os.WriteFile(inputFile, []byte(original), 0o644), "failed to create main.go")
			runGo(t, tmp, "mod", "tidy")

			if config.ImportPath == "" {
				config.ImportPath = testModuleName
			}

			astFile, err := parser.ParseFile(token.NewFileSet(), inputFile, []byte(original), parser.ParseComments)
			require.NoError(t, err, "failed to parse input file")

			importMap := make(map[string]string)
			for _, a := range astFile.Imports {
				ax, err := strconv.Unquote(a.Path.Value)
				require.NoError(t, err, "failed to unquote import path: %q", a.Path.Value)
				importMap[ax] = ""
			}

			inj := injector.Injector{
				ModifiedFile: func(path string) string { return filepath.Join(tmp, filepath.Base(path)+".edited.go") },
				ImportPath:   config.ImportPath,
				Lookup:       testLookup,
				ImportMap:    importMap,
			}

			res, resGoLang, err := inj.InjectFiles(gocontext.Background(), []string{inputFile}, config.Aspects)
			require.NoError(t, err, "failed to inject file")

			resFile, modified := res[inputFile]
			if !modified {
				golden.Assert(t, "<no changes>", filepath.Join(dirName, testName, "modified.go.snap"))
				return
			}

			assert.Equal(t, filepath.Join(tmp, filepath.Base(inputFile)+".edited.go"), resFile.Filename)
			assert.Equal(t, config.SyntheticReferences, resFile.References.Map())
			assert.Equal(t, config.GoLang.String(), resGoLang.String())

			modifiedData, err := os.ReadFile(resFile.Filename)
			require.NoError(t, err, "failed to read modified file")
			normalized := normalize(modifiedData, inputFile)

			golden.Assert(t, normalized, filepath.Join(dirName, testName, "modified.go.snap"))

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
