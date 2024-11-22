// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package config

import (
	"bytes"
	_ "embed" // For go:embed
	"fmt"
	"sync"

	"github.com/santhosh-tekuri/jsonschema/v6"
)

// ValidateObject checks the provided object for conformance to the embedded
// JSON schema. Returns an error if the object does not conform to the schema.
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
	rawSchema, err := jsonschema.UnmarshalJSON(bytes.NewReader(schemaBytes))
	if err != nil {
		panic(fmt.Errorf("parsing JSON schema: %w", err))
	}
	mapSchema, _ := rawSchema.(map[string]any)
	schemaURL, _ := mapSchema["$id"].(string)

	compiler := jsonschema.NewCompiler()
	if err := compiler.AddResource(schemaURL, rawSchema); err != nil {
		panic(fmt.Errorf("preparing JSON schema compiler: %w", err))
	}

	schema, err = compiler.Compile(schemaURL)
	if err != nil {
		panic(fmt.Errorf("compiling JSON schema: %w", err))
	}
}
