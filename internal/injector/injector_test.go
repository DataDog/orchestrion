// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package injector_test

import (
	gocontext "context"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io"
	"maps"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/DataDog/orchestrion/internal/injector"
	"github.com/DataDog/orchestrion/internal/injector/aspect"
	"github.com/DataDog/orchestrion/internal/injector/aspect/context"
	"github.com/DataDog/orchestrion/internal/injector/typed"
	"github.com/DataDog/orchestrion/internal/yaml"
	"github.com/dave/dst"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/tools/go/packages"
	"gotest.tools/v3/golden"
)

type fixtureFile struct {
	Code                string                         `yaml:"code"`
	SyntheticReferences map[string]typed.ReferenceKind `yaml:"syntheticReferences"`
}

type testConfig struct {
	Aspects             []*aspect.Aspect               `yaml:"aspects"`
	SyntheticReferences map[string]typed.ReferenceKind `yaml:"syntheticReferences"`
	GoLang              context.GoLangVersion          `yaml:"required-lang"`
	Code                string                         `yaml:"code"`
	ImportPath          string                         `yaml:"import-path"`

	// Files declares additional package files (support files) for fixtures that need a multi-file
	// package. The keys are file base names; "input.go" is reserved for the main fixture code.
	Files map[string]fixtureFile `yaml:"files"`

	// Modules declares local dependency modules. The keys are module paths; the values map file
	// base names to their content (go.mod is generated).
	Modules map[string]map[string]string `yaml:"modules"`

	// Repeat runs InjectFiles this many times and requires byte-identical output and identical
	// references on every run (determinism check, F07). Zero means once.
	Repeat int `yaml:"repeat"`

	// Vendor runs the module in vendor mode (D12).
	Vendor bool `yaml:"vendor"`
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

			cfgFile, err := os.Open(filepath.Join(testPath, "config.yml"))
			require.NoError(t, err, "failed to open test configuration")
			defer cfgFile.Close()
			var config testConfig
			require.NoError(t, yaml.UnmarshalContext(gocontext.Background(), cfgFile, &config), "failed to parse test configuration")

			runGo(t, tmp, "mod", "init", testModuleName)
			runGo(t, tmp, "mod", "edit",
				"-replace=github.com/DataDog/orchestrion="+rootDir,
				"-replace=github.com/DataDog/orchestrion/instrument="+filepath.Join(rootDir, "instrument"),
			)

			// Create local dependency modules, if any, in a deterministic order.
			for _, modulePath := range slices.Sorted(maps.Keys(config.Modules)) {
				dir := filepath.Join(tmp, "modules", sanitizePathElement(modulePath))
				writeLocalModule(t, dir, modulePath, config.Modules[modulePath])
				runGo(t, tmp, "mod", "edit", "-replace="+modulePath+"="+dir)
			}

			// Write the fixture package: input.go plus any support files.
			inputFile := filepath.Join(tmp, "input.go")
			require.NoError(t, os.WriteFile(inputFile, []byte(strings.TrimSpace(config.Code)+"\n"), 0o644), "failed to create input.go")

			supportFiles := make([]string, 0, len(config.Files))
			for _, name := range slices.Sorted(maps.Keys(config.Files)) {
				require.NotEqual(t, "input.go", name, "input.go is reserved for the top-level fixture code")
				path := filepath.Join(tmp, name)
				code := strings.TrimSpace(config.Files[name].Code) + "\n"
				require.NoError(t, os.WriteFile(path, []byte(code), 0o644), "failed to create support file")
				supportFiles = append(supportFiles, path)
			}
			allFiles := append([]string{inputFile}, supportFiles...)

			runGo(t, tmp, "mod", "tidy")
			// The original input must be valid Go: a fixture that fails here tests nothing, because
			// the injector skips files that do not type-check.
			runGo(t, tmp, "vet", "./...")
			if config.Vendor {
				runGo(t, tmp, "mod", "vendor")
			}

			if config.ImportPath == "" {
				config.ImportPath = testModuleName
			}

			// The import map is the union of the imports of all files of the package.
			importMap := make(map[string]string)
			for _, path := range allFiles {
				astFile, err := parser.ParseFile(token.NewFileSet(), path, nil, parser.ParseComments)
				require.NoError(t, err, "failed to parse input file %q", path)
				for _, a := range astFile.Imports {
					ax, err := strconv.Unquote(a.Path.Value)
					require.NoError(t, err, "failed to unquote import path: %q", a.Path.Value)
					importMap[ax] = ""
				}
			}

			// Build the binding oracle from the original sources, before any injection runs.
			oracle := newBindingOracle(t, allFiles, testLookup, config.ImportPath)
			declaredImports := declaredImportPaths(config.Aspects)

			inj := injector.Injector{
				ModifiedFile: func(path string) string { return filepath.Join(tmp, filepath.Base(path)+".edited.go") },
				ImportPath:   config.ImportPath,
				Lookup:       testLookup,
				ImportMap:    importMap,
			}

			// The capture hook records the intended binding of every package reference of every
			// modified file, straight from the restoration step (see WithRestoreCapture).
			env := bindingEnv{oracle: oracle, declaredImports: declaredImports, importPath: config.ImportPath}
			captured := make(map[string][]capturedRef)
			var captureMu sync.Mutex
			hook := func(filename string, file *dst.File, decoratorNodes, restorerNodes map[dst.Node]ast.Node, restored *ast.File, fset *token.FileSet) {
				refs, err := captureIntendedRefs(&env, captureArgs{
					filename:   filename,
					file:       file,
					decorNodes: decoratorNodes,
					resNodes:   restorerNodes,
					restored:   restored,
					fset:       fset,
				})
				captureMu.Lock()
				defer captureMu.Unlock()
				if err != nil {
					captured[filename] = append(captured[filename], capturedRef{invalid: err})
					return
				}
				captured[filename] = refs
			}

			repeat := config.Repeat
			if repeat < 1 {
				repeat = 1
			}
			var (
				res          map[string]injector.InjectedFile
				resGoLang    context.GoLangVersion
				firstBytes   map[string][]byte
				firstRefs    map[string]map[string]typed.ReferenceKind
				expectedRefs = func(filename string) map[string]typed.ReferenceKind {
					if filepath.Base(filename) == "input.go" {
						return config.SyntheticReferences
					}
					return config.Files[filepath.Base(filename)].SyntheticReferences
				}
			)
			for run := range repeat {
				clear(captured)
				ctx := injector.WithRestoreCapture(gocontext.Background(), hook)
				var err error
				res, resGoLang, err = inj.InjectFiles(ctx, allFiles, config.Aspects)
				require.NoError(t, err, "failed to inject files (run %d)", run)

				if run == 0 {
					firstBytes = make(map[string][]byte)
					firstRefs = make(map[string]map[string]typed.ReferenceKind)
					for filename, resFile := range res {
						data, err := os.ReadFile(resFile.Filename)
						require.NoError(t, err)
						firstBytes[filename] = data
						firstRefs[filename] = resFile.References.Map()
					}
					continue
				}
				// F07: repeated runs must produce byte-identical output and identical references.
				require.Len(t, res, len(firstBytes), "run %d modified a different set of files", run)
				for filename, resFile := range res {
					data, err := os.ReadFile(resFile.Filename)
					require.NoError(t, err)
					require.Equal(t, string(firstBytes[filename]), string(data), "run %d produced different output for %s", run, filename)
					require.Equal(t, firstRefs[filename], resFile.References.Map(), "run %d produced different references for %s", run, filename)
				}
			}

			resFile, modified := res[inputFile]
			if !modified {
				require.Empty(t, res, "no file may be modified if input.go is not")
				golden.Assert(t, "<no changes>", filepath.Join(dirName, testName, "modified.go.snap"))
				return
			}

			assert.Equal(t, filepath.Join(tmp, filepath.Base(inputFile)+".edited.go"), resFile.Filename)
			assert.Equal(t, expectedRefs(inputFile), resFile.References.Map())
			assert.Equal(t, config.GoLang.String(), resGoLang.String())

			for _, supportFile := range supportFiles {
				supportRes, supportModified := res[supportFile]
				if !supportModified {
					continue
				}
				assert.Equal(t, filepath.Join(tmp, filepath.Base(supportFile)+".edited.go"), supportRes.Filename)
				assert.Equal(t, expectedRefs(supportFile), supportRes.References.Map())
			}

			// Compare goldens for every modified file.
			for filename, resFile := range res {
				modifiedData, err := os.ReadFile(resFile.Filename)
				require.NoError(t, err, "failed to read modified file")
				golden.Assert(t, normalize(modifiedData, filename), filepath.Join(dirName, testName, goldenFileName(filename)))
			}

			// Replace the original files with their modified versions, then verify the result
			// builds and that every package reference keeps its intended binding.
			for original, resFile := range res {
				require.NoError(t, os.Rename(resFile.Filename, original))
			}
			runGo(t, tmp, "mod", "tidy")
			if config.Vendor {
				runGo(t, tmp, "mod", "vendor")
			}

			checkPackageBindings(t, allFiles, captured, testLookup, config.ImportPath)

			if len(allFiles) == 1 {
				runGo(t, tmp, "build", inputFile)
			} else {
				runGo(t, tmp, "build", ".")
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

// writeLocalModule creates a local module at dir with the given module path and files.
func writeLocalModule(t *testing.T, dir string, modulePath string, files map[string]string) {
	t.Helper()

	require.NoError(t, os.MkdirAll(dir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "go.mod"), []byte(fmt.Sprintf("module %s\n\ngo 1.26\n", modulePath)), 0o644))
	for _, name := range slices.Sorted(maps.Keys(files)) {
		path := filepath.Join(dir, name)
		require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
		require.NoError(t, os.WriteFile(path, []byte(files[name]), 0o644))
	}
}

func sanitizePathElement(path string) string {
	return strings.NewReplacer("/", "_", ".", "_", "-", "_").Replace(path)
}

// goldenFileName returns the golden snapshot file name for a fixture source file. The main
// fixture file (input.go) keeps the historical "modified.go.snap" name; support files get one
// snapshot each.
func goldenFileName(filename string) string {
	base := filepath.Base(filename)
	if base == "input.go" {
		return "modified.go.snap"
	}
	return fmt.Sprintf("modified.%s.snap", strings.TrimSuffix(base, ".go"))
}

// normalize replaces all tabulation characters with two spaces, matching the indentation style found in YAML documents,
// and cleans up line directives to remove the temporary file name.
func normalize(in []byte, filename string) string {
	res := strings.ReplaceAll(string(in), "\t", "  ")
	res = strings.ReplaceAll(res, fmt.Sprintf("//line %s:", filename), fmt.Sprintf("//line %s:", filepath.Base(filename)))
	return res
}
