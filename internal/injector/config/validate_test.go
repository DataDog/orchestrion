// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package config

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/santhosh-tekuri/jsonschema/v6"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func TestBuiltinYAML(t *testing.T) {
	_, thisFile, _, _ := runtime.Caller(0)
	rootDir := filepath.Join(thisFile, "..", "..", "..", "..")

	yamlSegment := fmt.Sprintf("%[1]cyaml%[1]c", filepath.Separator)

	count := 0
	filepath.WalkDir(rootDir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if d.IsDir() {
			// Ignore `testdata` directories...
			if d.Name() == "testdata" {
				return filepath.SkipDir
			}
			// Ignores `.git` and other hidden directories...
			if strings.HasPrefix(d.Name(), ".") {
				return filepath.SkipDir
			}
			return nil
		}

		if filepath.Ext(path) != ".yml" {
			// Only interested in .yml files that aren't hidden files...
			return nil
		}

		if d.Name() != FilenameOrchestrionYML && !strings.Contains(path, yamlSegment) {
			// Only look at `.yml` files that aren't `orchestrion.yml` if they're
			// under a `yaml` directory...
			return nil
		}

		count++
		rel, err := filepath.Rel(rootDir, path)
		require.NoError(t, err)
		t.Run(rel, func(t *testing.T) {
			file, err := os.Open(path)
			require.NoError(t, err)
			defer func() { require.NoError(t, file.Close()) }()

			var raw map[string]any
			require.NoError(t, yaml.NewDecoder(file).Decode(&raw))
			require.NoError(t, ValidateObject(raw))
		})

		return nil
	})

	require.Positive(t, count)
}

func TestSchemaValidity(t *testing.T) {
	count := validateExamples(t, getSchema(), "", nil)
	// Make sure we verified some examples...
	require.Greater(t, count, 30)
}

func validateExamples(t *testing.T, schema *jsonschema.Schema, path string, visited map[*jsonschema.Schema]struct{}) int {
	if schema == nil {
		return 0
	}

	if _, dup := visited[schema]; dup {
		return 0
	}
	if visited == nil {
		visited = make(map[*jsonschema.Schema]struct{})
	}
	visited[schema] = struct{}{}

	for idx, example := range schema.Examples {
		require.NoError(t, schema.Validate(example), "invalid example at %s.Examples[%d]", path, idx)
	}
	count := len(schema.Examples)

	count += validateExamples(t, schema.Ref, path+".Ref", visited)
	count += validateExamples(t, schema.RecursiveRef, path+".RecursiveRef", visited)
	if schema.DynamicRef != nil {
		count += validateExamples(t, schema.DynamicRef.Ref, path+".DynamicRef.Ref", visited)
	}
	count += validateExamples(t, schema.Not, path+".Not", visited)
	count += validateExamplesList(t, schema.AllOf, path+".AllOf", visited)
	count += validateExamplesList(t, schema.AnyOf, path+".AnyOf", visited)
	count += validateExamplesList(t, schema.OneOf, path+".OneOf", visited)
	count += validateExamples(t, schema.If, path+".If", visited)
	count += validateExamples(t, schema.Then, path+".Then", visited)
	count += validateExamples(t, schema.Else, path+".Else", visited)
	count += validateExamples(t, schema.PropertyNames, path+".PropertyNames", visited)
	count += validateExamplesMap(t, schema.Properties, path+".Properties", visited)
	count += validateExamplesMap(t, schema.PatternProperties, path+".PatternProperties", visited)
	count += validateExamplesMap(t, schema.DependentSchemas, path+".DependentSchemas", visited)
	count += validateExamples(t, schema.UnevaluatedProperties, path+".UnevaluatedProperties", visited)
	count += validateExamples(t, schema.Contains, path+".Contains", visited)
	count += validateExamplesList(t, schema.PrefixItems, path+".PrefixItems", visited)
	count += validateExamples(t, schema.Items2020, path+".Items2020", visited)
	count += validateExamples(t, schema.UnevaluatedItems, path+".UnevaluatedItems", visited)
	count += validateExamples(t, schema.ContentSchema, path+".ContentSchema", visited)

	return count
}

func validateExamplesList(t *testing.T, schemas []*jsonschema.Schema, path string, visited map[*jsonschema.Schema]struct{}) int {
	count := 0
	for idx, schema := range schemas {
		count += validateExamples(t, schema, fmt.Sprintf("%s[%d]", path, idx), visited)
	}
	return count
}

func validateExamplesMap[K comparable](t *testing.T, schemas map[K]*jsonschema.Schema, path string, visited map[*jsonschema.Schema]struct{}) int {
	count := 0
	for key, schema := range schemas {
		count += validateExamples(t, schema, fmt.Sprintf("%s[%v]", path, key), visited)
	}
	return count
}
