// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package config

import (
	_ "embed" // For go:embed
	"encoding/json"
	"fmt"
	"sync"

	"github.com/santhosh-tekuri/jsonschema/v6"
)

// Validate checks the provided object for conformance to the embedded JSON
// schema. Returns an error if the object does not conform to the schema.
func ValidateObject(obj map[string]any) error {
	return getSchema().Validate(obj)
}

var (
	//go:embed "schema.json"
	schemaBytes []byte
	schema      *jsonschema.Schema
	schemaOnce  sync.Once
)

func getSchema() *jsonschema.Schema {
	schemaOnce.Do(compileSchema)
	return schema
}

func compileSchema() {
	var rawSchema map[string]any
	if err := json.Unmarshal(schemaBytes, &rawSchema); err != nil {
		panic(fmt.Errorf("parsing JSON schema: %w", err))
	}
	schemaURL, _ := rawSchema["$id"].(string)

	compiler := jsonschema.NewCompiler()
	if err := compiler.AddResource(schemaURL, rawSchema); err != nil {
		panic(fmt.Errorf("preparing JSON schema compiler: %w", err))
	}

	var err error
	schema, err = compiler.Compile(schemaURL)
	if err != nil {
		panic(fmt.Errorf("compiling JSON schema: %w", err))
	}
}
