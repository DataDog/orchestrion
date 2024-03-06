// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package injector

import (
	_ "embed" // For go:embed
	"testing"

	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

//go:embed "testdata/postprocess.yml"
var postProcessTestdata []byte

func TestPostProcess(t *testing.T) {
	type testCase struct {
		Source   string `yaml:"source"`
		Expected string `yaml:"expected"`
	}
	var cases map[string]testCase
	err := yaml.Unmarshal(postProcessTestdata, &cases)
	require.NoError(t, err, "failed to parse test suite data")

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			actual := postProcess([]byte(tc.Source))
			require.Equal(t, tc.Expected, string(actual))
		})
	}
}
