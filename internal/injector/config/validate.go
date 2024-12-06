// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package config

import (
	_ "embed" // For go:embed
	"errors"
	"fmt"
	"sync"

	"github.com/xeipuuv/gojsonschema"
)

// ValidateObject checks the provided object for conformance to the embedded
// JSON schema. Returns an error if the object does not conform to the schema.
func ValidateObject(obj map[string]any) error {
	res, err := getSchema().Validate(gojsonschema.NewGoLoader(obj))
	if err != nil {
		return fmt.Errorf("unknown object type for schema validation: %w", err)
	}

	if !res.Valid() {
		errs := make([]error, len(res.Errors()))
		for i, err := range res.Errors() {
			errs[i] = fmt.Errorf("error at %s: %s", err.Field(), err.Description())
		}
		return fmt.Errorf("object does not conform to schema: %w", errors.Join(errs...))
	}

	return nil
}

var (
	//go:embed "schema.json"
	schemaBytes []byte
	schema      *gojsonschema.Schema
	schemaOnce  sync.Once
)

func getSchema() *gojsonschema.Schema {
	schemaOnce.Do(compileSchema)
	return schema
}

func compileSchema() {
	var err error
	schema, err = gojsonschema.NewSchema(gojsonschema.NewBytesLoader(schemaBytes))
	if err != nil {
		panic(fmt.Errorf("compiling JSON schema: %w", err))
	}
}
