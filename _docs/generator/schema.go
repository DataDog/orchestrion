// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package main

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/santhosh-tekuri/jsonschema/v6"
	"gopkg.in/yaml.v3"
)

func documentSchema(dir string) error {
	joinPointSchema, adviceSchema, err := loadSchema()
	if err != nil {
		return err
	}

	joinDir := filepath.Join(dir, "join-points")
	if err := os.RemoveAll(joinDir); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if err := os.MkdirAll(joinDir, 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(joinDir, "_index.md"), []byte("---\ntitle: Join Points\ntype: docs\nweight: 1\n---\n\n{{<menu icon=\"search-circle\">}}\n"), 0o644); err != nil {
		return err
	}
	for _, jp := range joinPointSchema.OneOf {
		jp = jp.Ref
		name := jp.Location[strings.LastIndex(jp.Location, "/")+1:]
		if err := documentSchemaInstance(jp, filepath.Join(joinDir, name+".md")); err != nil {
			return err
		}
	}

	advDir := filepath.Join(dir, "advice")
	if err := os.RemoveAll(advDir); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if err := os.MkdirAll(advDir, 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(advDir, "_index.md"), []byte("---\ntitle: Advice\ntype: docs\nweight: 2\n---\n\n{{<menu icon=\"pencil\">}}\n"), 0o644); err != nil {
		return err
	}
	for _, adv := range adviceSchema.OneOf {
		adv := adv.Ref
		name := adv.Location[strings.LastIndex(adv.Location, "/")+1:]
		if err := documentSchemaInstance(adv, filepath.Join(advDir, name+".md")); err != nil {
			return err
		}
	}

	return nil
}

func documentSchemaInstance(schema *jsonschema.Schema, path string) error {
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()

	var (
		propName string
		prop     *jsonschema.Schema
	)
	for name, p := range schema.Properties {
		propName = name
		prop = p
		break
	}
	if prop == nil {
		return errors.New("unexpected schema - no properties?")
	}

	_, _ = fmt.Fprintln(file, "---")
	_, _ = fmt.Fprintf(file, "title: %q\n", propName)
	_, _ = fmt.Fprintf(file, "subtitle: %q\n", prop.Title)
	_, _ = fmt.Fprintln(file, "type: docs")
	_, _ = fmt.Fprintln(file, "---")
	_, _ = fmt.Fprintln(file)

	_, _ = fmt.Fprintln(file, prop.Description)
	_, _ = fmt.Fprintln(file, "<!--more-->")

	if prop.Deprecated {
		_, _ = fmt.Fprintln(file, `{{<callout type="warning">}}`)
		_, _ = fmt.Fprintln(file, "This feature is deprecated and should not be used in new configurations, as it may be")
		_, _ = fmt.Fprintln(file, "removed in future versions of Orchestrion.")
		_, _ = fmt.Fprintln(file, `{{</callout>}}`)
	}
	_, _ = fmt.Fprintln(file)

	if len(schema.Examples) > 0 {
		_, _ = fmt.Fprintln(file, "## Examples")
		_, _ = fmt.Fprintln(file)
		for idx, ex := range schema.Examples {
			if err := schema.Validate(ex); err != nil {
				return fmt.Errorf("invalid example (index %d): %w", idx, err)
			}

			yml, err := yaml.Marshal(ex)
			if err != nil {
				return err
			}
			yml = bytes.TrimSuffix(yml, []byte("\n"))
			_, _ = fmt.Fprintf(file, "```yaml\n%s\n```\n", string(yml))
		}
	}

	return nil
}

func loadSchema() (*jsonschema.Schema, *jsonschema.Schema, error) {
	_, thisFile, _, _ := runtime.Caller(0)
	schemaFile := filepath.Join(thisFile, "..", "..", "..", "internal", "injector", "config", "schema.json")

	file, err := os.Open(schemaFile)
	if err != nil {
		return nil, nil, err
	}
	defer file.Close()

	json, err := jsonschema.UnmarshalJSON(file)
	if err != nil {
		return nil, nil, err
	}

	json = normalizeSchema(json)

	schemaUrl := "https://datadoghq.dev/orchestrion/schema.json"
	compiler := jsonschema.NewCompiler()

	if err := compiler.AddResource(schemaUrl, json); err != nil {
		return nil, nil, err
	}
	joinPointSchema := compiler.MustCompile(schemaUrl + "#/$defs/JoinPoint")
	adviceSchema := compiler.MustCompile(schemaUrl + "#/$defs/Advice")

	return joinPointSchema, adviceSchema, nil
}

func normalizeSchema(original any) any {
	switch original := original.(type) {
	case map[string]any:
		// Copy `markdownDescription` to `description` because the jsonschema library doesn't support
		// the `markdownDescription` attribute, despite it is common in IDE validators... And since we
		// render as markdown anyway...
		if md := original["markdownDescription"]; md != nil {
			if _, hasDesc := original["description"]; !hasDesc {
				original["description"] = md
			}
		}
		// Normalize all the nested sub-schemas.
		for key, val := range original {
			original[key] = normalizeSchema(val)
		}
	case []any:
		for i, item := range original {
			original[i] = normalizeSchema(item)
		}
	default:
		// Nothing to do...
	}
	return original
}
