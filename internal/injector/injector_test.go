// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package injector_test

import (
	_ "embed" // For go:embed
	"fmt"
	"os"
	"os/exec"
	"path"
	"runtime"
	"strings"
	"testing"

	"github.com/datadog/orchestrion/internal/injector"
	"github.com/datadog/orchestrion/internal/injector/typed"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

//go:embed "testdata/injector.yml"
var injectorTestdata []byte

func TestInjector(t *testing.T) {
	_, filename, _, _ := runtime.Caller(0)
	orchestrionPath := path.Dir(path.Dir(path.Dir(filename)))

	var goModFile = []byte(strings.Join([]string{
		"module dummy/test/module",
		"",
		"go 1.19",
		"",
		"require (",
		"\tgithub.com/datadog/orchestrion v0.0.0",
		"\tgithub.com/go-chi/chi/v5 v5.0.10",
		"\torchestrion/integration v0.0.0",
		")",
		"replace (",
		fmt.Sprintf("\tgithub.com/datadog/orchestrion => %q", orchestrionPath),
		fmt.Sprintf("\torchestrion/integration => %q", path.Join(orchestrionPath, "_integration-tests")),
		")",
	}, "\n"))
	type testCase struct {
		Options  injector.InjectorOptions `yaml:"options"`
		Source   string                   `yaml:"source"`
		Expected struct {
			Modified   bool               `yaml:"modified"`
			References typed.ReferenceMap `yaml:"references"`
			Source     string             `yaml:"source"`
		} `yaml:"expected"`
	}
	var cases map[string]testCase
	err := yaml.Unmarshal(injectorTestdata, &cases)
	require.NoError(t, err, "failed to parse test suite data")

	t.Parallel()

	for name, tc := range cases {
		if name != "database-sql" {
			continue
		}

		t.Run(name, func(t *testing.T) {
			tc.Options.ModifiedFile = func(filename string) string { return filename + ".edited" }

			dir, err := os.MkdirTemp("", fmt.Sprintf("orchestrion-injector-test-*-%s", name))
			require.NoError(t, err, "failed to create temporary directory")
			defer os.RemoveAll(dir)

			require.NoError(t, os.WriteFile(path.Join(dir, "go.mod"), goModFile, 0o644), "failed to write go.mod file")

			require.NoError(t, os.Mkdir(path.Join(dir, "main"), 0o755), "failed to create main directory")
			require.NoError(t, os.WriteFile(path.Join(dir, "tools.go"), []byte("//go:build tools\npackage tools\nimport _ \"github.com/datadog/orchestrion/instrument\""), 0o644), "failed to write tools.go file")

			filename := path.Join(dir, "main", "input.go")
			require.NoError(t, os.WriteFile(filename, []byte(tc.Source), 0o644), "failed to write injection input file")

			run := func(cmd string, args ...string) error {
				child := exec.Command(cmd, args...)
				child.Dir = dir
				child.Stdin = os.Stdin
				child.Stdout = os.Stdout
				child.Stderr = os.Stderr
				return child.Run()
			}
			require.NoError(t, run("go", "mod", "tidy"), "failed to run go mod tidy")
			require.NoError(t, run("go", "mod", "download"), "failed to run go mod download")

			injector, err := injector.NewInjector(path.Dir(filename), tc.Options)
			require.NoError(t, err)
			res, err := injector.InjectFile(filename)
			require.NoError(t, err)
			assert.Equal(t, tc.Expected.Modified, res.Modified, "modified status")
			assert.Equal(t, tc.Expected.References, res.References)

			if res.Modified {
				assert.Equal(t, filename+".edited", res.Filename, "output filename")

				out, err := os.ReadFile(res.Filename)
				require.NoError(t, err, "failed to read injection output file")
				assert.Equal(t, tc.Expected.Source, normalize(out, filename), "injected output")
			}
		})
	}
}

// normalize replaces all tabulation characters with two spaces, matching the indentation style found in YAML documents,
// and cleans up line directives to remove the temporary file name.
func normalize(in []byte, filename string) string {
	res := strings.ReplaceAll(string(in), "\t", "  ")
	res = strings.ReplaceAll(res, fmt.Sprintf("//line %s:", filename), "//line input.go:")
	return res
}
