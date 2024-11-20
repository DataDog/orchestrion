// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package main

import (
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/DataDog/orchestrion/internal/injector/aspect"
	"github.com/santhosh-tekuri/jsonschema/v6"
	"gopkg.in/yaml.v3"
)

type ConfigurationFile struct {
	Metadata Metadata `yaml:"meta"`
	Aspects  []IdentifiedAspect
}

type Metadata struct {
	Name        string
	Description string
	Icon        string `yaml:",omitempty"`
	Caveats     string `yaml:",omitempty"`
}

type IdentifiedAspect struct {
	ID     string        `yaml:"id"`
	Aspect aspect.Aspect `yaml:",inline"`
}

var (
	configSchema    *jsonschema.Schema
	joinPointSchema *jsonschema.Schema
	adviceSchema    *jsonschema.Schema
)

func documentSchema(dir string) error {
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

	fmt.Fprintln(file, "---")
	fmt.Fprintf(file, "title: %q\n", propName)
	fmt.Fprintf(file, "subtitle: %q\n", prop.Title)
	fmt.Fprintln(file, "type: docs")
	fmt.Fprintln(file, "---")
	fmt.Fprintln(file)

	fmt.Fprintln(file, prop.Description)
	fmt.Fprintln(file, "<!--more-->")

	if prop.Deprecated {
		fmt.Fprintln(file, `{{<callout type="warning">}}`)
		fmt.Fprintln(file, "This feature is deprecated and should not be used in new configurations, as it may be")
		fmt.Fprintln(file, "removed in future versions of Orchestrion.")
		fmt.Fprintln(file, `{{</callout>}}`)
	}
	fmt.Fprintln(file)

	if len(schema.Examples) > 0 {
		fmt.Fprintln(file, "## Examples")
		fmt.Fprintln(file)
		for idx, ex := range schema.Examples {
			if err := schema.Validate(ex); err != nil {
				return fmt.Errorf("invalid example (index %d): %w", idx, err)
			}

			yml, err := yaml.Marshal(ex)
			if err != nil {
				return err
			}
			fmt.Fprintf(file, "```yaml\n%s\n```\n", string(yml))
		}
	}

	return nil
}

// validateSchema verifies the YAML data matches the expected JSON Schema definition.
func validateSchema(data []byte) error {
	var obj any
	if err := yaml.Unmarshal(data, &obj); err != nil {
		return err
	}
	return configSchema.Validate(obj)
}

func init() {
	_, thisFile, _, _ := runtime.Caller(0)
	schemaFile := filepath.Join(thisFile, "..", "..", "..", "..", "..", "_docs", "static", "schema.json")

	file, err := os.Open(schemaFile)
	if err != nil {
		log.Fatalln(err)
	}
	defer file.Close()

	json, err := jsonschema.UnmarshalJSON(file)
	if err != nil {
		log.Fatalln(err)
	}

	json = normalizeSchema(json)

	schemaUrl := "https://datadoghq.dev/orchestrion/schema.json"
	compiler := jsonschema.NewCompiler()

	if err := compiler.AddResource(schemaUrl, json); err != nil {
		log.Fatalln(err)
	}
	configSchema = compiler.MustCompile(schemaUrl)
	joinPointSchema = compiler.MustCompile(schemaUrl + "#/$defs/JoinPoint")
	adviceSchema = compiler.MustCompile(schemaUrl + "#/$defs/Advice")
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
