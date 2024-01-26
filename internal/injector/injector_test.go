// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package injector_test

import (
	_ "embed" // For go:embed
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/datadog/orchestrion/internal/injector"
	"github.com/datadog/orchestrion/internal/injector/typed"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

//go:embed "testdata/injector.yml"
var injectorTestdata []byte

func TestInjector(t *testing.T) {
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

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			tc.Options.ModifiedFile = func(filename string) string { return filename + ".edited" }

			file, err := os.CreateTemp("", fmt.Sprintf("orchestrion-injector-test-*-%s-input.go", name))
			require.NoError(t, err, "failed to create temporary file for injection input")
			require.NoError(t, file.Close(), "failed to close injection input temporary file")
			filename := file.Name()
			defer os.Remove(filename) // Clean up after ourselves

			require.NoError(t, os.WriteFile(filename, []byte(tc.Source), 0o644), "failed to write injection input file")

			res, err := injector.NewInjector(tc.Options).InjectFile(filename)
			require.NoError(t, err)
			require.Equal(t, tc.Expected.Modified, res.Modified, "modified status")
			require.Equal(t, tc.Expected.References, res.References)

			if res.Modified {
				require.Equal(t, filename+".edited", res.Filename, "output filename")

				out, err := os.ReadFile(res.Filename)
				require.NoError(t, err, "failed to read injection output file")
				require.Equal(t, tc.Expected.Source, normalize(out, filename), "injected output")
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
