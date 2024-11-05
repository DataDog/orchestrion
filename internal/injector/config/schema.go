// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package config

import (
	"encoding/json"
	"fmt"
	"io"
	"sync"

	"github.com/dlclark/regexp2"
	"github.com/santhosh-tekuri/jsonschema/v6"
	"gopkg.in/yaml.v3"

	_ "embed" // For go:embed
)

var (
	//go:embed schema.json
	jsonSchema []byte
	yamlSchema *jsonschema.Schema
	once       sync.Once
)

func Validate(reader io.Reader) error {
	var obj map[string]any
	if err := yaml.NewDecoder(reader).Decode(&obj); err != nil {
		return err
	}
	return ValidateObject(obj)
}

func ValidateObject(obj map[string]any) error {
	return getSchema().Validate(obj)
}

func getSchema() *jsonschema.Schema {
	once.Do(func() {
		compiler := jsonschema.NewCompiler()
		compiler.UseRegexpEngine(regexpEngine)

		var rawSchema map[string]any
		if err := json.Unmarshal(jsonSchema, &rawSchema); err != nil {
			panic(fmt.Errorf("decoding JSON schema: %w", err))
		}

		schemaURL, _ := rawSchema["$id"].(string)
		if err := compiler.AddResource(schemaURL, rawSchema); err != nil {
			panic(fmt.Errorf("adding resource to jsonschema compiler: %w", err))
		}
		schema, err := compiler.Compile(schemaURL)
		if err != nil {
			panic(fmt.Errorf("compiling jsonschema: %w", err))
		}
		yamlSchema = schema
	})
	return yamlSchema
}

type re2 regexp2.Regexp

func (re *re2) MatchString(s string) bool {
	matched, err := (*regexp2.Regexp)(re).MatchString(s)
	return err == nil && matched
}

func (re *re2) String() string {
	return (*regexp2.Regexp)(re).String()
}

func regexpEngine(s string) (jsonschema.Regexp, error) {
	re, err := regexp2.Compile(s, regexp2.ECMAScript)
	if err != nil {
		return nil, err
	}
	return (*re2)(re), nil
}
