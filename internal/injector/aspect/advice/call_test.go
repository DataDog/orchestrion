// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package advice_test

import (
	"testing"

	"github.com/DataDog/orchestrion/internal/injector/aspect/advice"
	"github.com/DataDog/orchestrion/internal/injector/aspect/advice/code"
	"github.com/DataDog/orchestrion/internal/injector/aspect/context"
	"github.com/DataDog/orchestrion/internal/injector/aspect/join"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAppendArgs(t *testing.T) {
	t.Run("AddedImports", func(t *testing.T) {
		type testCase struct {
			argType         join.TypeName
			args            []code.Template
			expectedImports []string
		}

		testCases := map[string]testCase{
			"imports-none": {
				argType: join.MustTypeName("any"),
				args:    []code.Template{code.MustTemplate("true", nil, context.GoLangVersion{})},
			},
			"imports-from-arg-type": {
				argType:         join.MustTypeName("*net/http.Request"),
				args:            []code.Template{code.MustTemplate("true", nil, context.GoLangVersion{})},
				expectedImports: []string{"net/http"},
			},
			"imports-from-templates": {
				argType: join.MustTypeName("any"),
				args: []code.Template{
					code.MustTemplate("imp.Value", map[string]string{"imp": "github.com/namespace/foo"}, context.GoLangVersion{}),
					code.MustTemplate("imp.Value", map[string]string{"imp": "github.com/namespace/bar"}, context.GoLangVersion{}),
				},
				expectedImports: []string{
					"github.com/namespace/foo",
					"github.com/namespace/bar",
				},
			},
		}

		for name, tc := range testCases {
			t.Run(name, func(t *testing.T) {
				imports := advice.AppendArgs(tc.argType, tc.args...).AddedImports()
				for _, imp := range tc.expectedImports {
					assert.Contains(t, imports, imp)
				}
				require.Equal(t, len(tc.expectedImports), len(imports), "expected %d imports, got %d", len(tc.expectedImports), len(imports))
			})
		}
	})
}
