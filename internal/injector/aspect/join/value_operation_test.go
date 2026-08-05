// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package join

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Test_SchemaEnumListsEveryValueOperation guards the drift that silently broke
// configuration validation once already: a new value operation was added to
// [allValueOperations] and used by runtime/taint/instrument/orchestrion.yml, but the
// `value-operation` enum in the JSON schema was never extended, so the integration
// stopped validating. Comparing both directions catches an addition on either side.
func Test_SchemaEnumListsEveryValueOperation(t *testing.T) {
	// Given
	schemaPath := filepath.Join("..", "..", "config", "schema.json")
	contents, err := os.ReadFile(schemaPath)
	require.NoError(t, err, "read %s", schemaPath)

	var schema struct {
		Defs struct {
			JoinPoint struct {
				ValueOperation struct {
					Properties struct {
						ValueOperation struct {
							Enum []string `json:"enum"`
						} `json:"value-operation"`
					} `json:"properties"`
				} `json:"value-operation"`
			} `json:"join-point"`
		} `json:"$defs"`
	}
	require.NoError(t, json.Unmarshal(contents, &schema), "decode %s", schemaPath)

	// When
	enum := schema.Defs.JoinPoint.ValueOperation.Properties.ValueOperation.Enum
	require.NotEmpty(t, enum, "schema declares no value-operation enum; the $defs path moved")

	inSchema := make(map[string]struct{}, len(enum))
	for _, name := range enum {
		inSchema[name] = struct{}{}
	}

	// Then
	for _, operation := range allValueOperations {
		assert.Contains(t, inSchema, string(operation),
			"value operation %q is accepted by the unmarshaler but missing from the schema enum in %s", operation, schemaPath)
	}
	for _, name := range enum {
		assert.Contains(t, supportedValueOperations, valueOperation(name),
			"schema enum in %s lists %q, which the unmarshaler rejects", schemaPath, name)
	}
	assert.Len(t, enum, len(allValueOperations), "schema enum and allValueOperations differ in size")
	assert.Len(t, supportedValueOperations, len(allValueOperations), "allValueOperations contains a duplicate")
}
