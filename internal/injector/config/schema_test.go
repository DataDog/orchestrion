// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package config

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/santhosh-tekuri/jsonschema/v6"
	"github.com/stretchr/testify/require"
)

func TestSelfValidate(t *testing.T) {
	visitor{T: t}.visit(getSchema(), "")
}

func TestValidate(t *testing.T) {
	t.Parallel()

	_, thisFile, _, _ := runtime.Caller(0)
	rootDir := filepath.Join(thisFile, "..", "..", "..", "..")

	filepath.WalkDir(rootDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		relPath, err := filepath.Rel(rootDir, path)
		require.NoError(t, err)

		if d.IsDir() {
			if relPath == "docs" || strings.HasPrefix(relPath, ".git") || d.Name() == "testdata" {
				return filepath.SkipDir
			}
			return nil
		}

		if filepath.Ext(path) != ".yml" {
			return nil
		}

		if d.Name() != "orchestrion.yml" && !strings.HasPrefix(relPath, filepath.Join("internal", "injector", "builtin", "yaml")) {
			return nil
		}

		t.Run(relPath, func(t *testing.T) {
			t.Parallel()

			file, err := os.Open(path)
			require.NoError(t, err)
			defer file.Close()
			require.NoError(t, Validate(file))
		})

		return nil
	})
}

type visitor struct {
	*testing.T
	visited map[*jsonschema.Schema]struct{}
}

func (v visitor) visit(schema *jsonschema.Schema, path string) {
	if schema == nil {
		return
	}

	if _, ok := v.visited[schema]; ok {
		return
	}
	if v.visited == nil {
		v.visited = make(map[*jsonschema.Schema]struct{})
	}
	v.visited[schema] = struct{}{}

	for i, example := range schema.Examples {
		require.NoError(v.T, schema.Validate(example), "example %v[%d]", path, i)
	}

	// Type agnostic
	v.visit(schema.Ref, path+".Ref")
	v.visit(schema.RecursiveRef, path+".RecursiveRef")
	if schema.DynamicRef != nil {
		v.visit(schema.DynamicRef.Ref, path+".DynamicRef.Ref")
	}
	v.visit(schema.Not, path+".Not")
	v.visitList(schema.AllOf, path+".AllOf")
	v.visitList(schema.AnyOf, path+".AnyOf")
	v.visitList(schema.OneOf, path+".OneOf")
	v.visit(schema.If, path+".If")
	v.visit(schema.Then, path+".Then")
	v.visit(schema.Else, path+".Else")

	// Object
	v.visit(schema.PropertyNames, path+".PropertyNames")
	v.visitStringMap(schema.Properties, path+".Properties")
	v.visitRegexMap(schema.PatternProperties, path+".PatternProperties")
	v.visitStringMap(schema.DependentSchemas, path+".DependentSchemas")
	v.visit(schema.UnevaluatedProperties, path+".UnevaluatedProperties")

	// Array
	v.visit(schema.Contains, path+".Contains")
	v.visitList(schema.PrefixItems, path+".PrefixItems")
	v.visit(schema.Items2020, path+".Items2020")
	v.visit(schema.UnevaluatedItems, path+".UnevaluatedItems")

	// String
	v.visit(schema.ContentSchema, path+".ContentSchema")
}

func (v visitor) visitList(list []*jsonschema.Schema, path string) {
	for i, schema := range list {
		v.visit(schema, fmt.Sprintf("%s[%d]", path, i))
	}
}

func (v visitor) visitRegexMap(m map[jsonschema.Regexp]*jsonschema.Schema, path string) {
	for key, schema := range m {
		v.visit(schema, fmt.Sprintf("%s[%s]", path, key))
	}
}

func (v visitor) visitStringMap(m map[string]*jsonschema.Schema, path string) {
	for key, schema := range m {
		v.visit(schema, fmt.Sprintf("%s.%s", path, key))
	}
}
