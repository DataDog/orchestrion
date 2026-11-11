// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package main

import (
	"strings"
	"testing"

	"github.com/santhosh-tekuri/jsonschema/v6"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The rendered documentation pages are generated from the source schema, so
// these exact descriptions are the published contract for generic call
// matching and replacement. The review step verifies the wording; this test
// detects unintended changes to it.
const (
	wantFunctionCallDescription = "The `function-call` join point matches call nodes whose callee is the specified symbol. It only matches `Call` nodes. It matches generic calls with inferred type arguments and direct calls with explicit type arguments, such as `F[T](...)` and `pkg.F[T1, T2](...)`. Explicit calls match when the type checker confirms that the operand is a generic function. When that type information is unavailable, an indexed callee does not match. Ordinary indexing of function values and generic type conversions do not match this explicit-call path. Parenthesized callees, indirect calls through stored function values, and methods of generic types do not match. Advice receives the complete call, including its explicit type arguments."

	wantReplaceFunctionDescription = "The `replace-function` advice replaces the callee of a `Call` node, typically matched by `function-call`, with the designated function. The original and replacement functions must have compatible signatures. For an explicit instantiation that the type checker confirms is a generic function call, the advice replaces only the function operand. It preserves the explicit type arguments in their original order. The replacement must accept those arguments and satisfy its type constraints. Any remaining type parameters must be inferable from the call arguments. A non-generic replacement cannot accept retained type arguments, even if its value signature matches the instantiated function. Incompatible replacements produce invalid Go code that the compiler rejects. For other callees, including ordinary value indexes and indexed callees without type information, the advice replaces the whole callee. The advice does not infer, remove, or reorder explicit type arguments."
)

// schemaByName returns the schema referenced by the oneOf entry of root whose
// location ends with "/"+name, e.g. the "function-call" join point.
func schemaByName(t *testing.T, root *jsonschema.Schema, name string) *jsonschema.Schema {
	t.Helper()

	for _, candidate := range root.OneOf {
		ref := candidate.Ref
		require.NotNil(t, ref, "oneOf entry without a $ref")
		if strings.HasSuffix(ref.Location, "/"+name) {
			return ref
		}
	}
	require.FailNow(t, "no $defs entry named %q", name)
	return nil
}

// markdownDescription asserts the property documentation the generator renders.
func markdownDescription(t *testing.T, schema *jsonschema.Schema) string {
	t.Helper()

	require.Len(t, schema.Properties, 1, "expected exactly one property")
	var prop *jsonschema.Schema
	for _, p := range schema.Properties {
		prop = p
		break
	}
	require.NotNil(t, prop)
	return prop.Description
}

func TestFunctionCallSchemaDocumentation(t *testing.T) {
	joinPointSchema, adviceSchema, err := loadSchema()
	require.NoError(t, err)

	assert.Equal(t,
		wantFunctionCallDescription,
		markdownDescription(t, schemaByName(t, joinPointSchema, "function-call")),
		"the function-call schema description must document type-guarded explicit generic calls",
	)

	assert.Equal(t,
		wantReplaceFunctionDescription,
		markdownDescription(t, schemaByName(t, adviceSchema, "replace-function")),
		"the replace-function schema description must document operand-only replacement",
	)
}
