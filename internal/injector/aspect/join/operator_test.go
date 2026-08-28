// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package join

import (
	gocontext "context"
	"testing"

	"github.com/goccy/go-yaml"
	"github.com/stretchr/testify/require"

	"github.com/DataDog/orchestrion/internal/fingerprint"
)

func TestStringConcatBoundsAndHash(t *testing.T) {
	first, err := StringConcat(2, 16)
	require.NoError(t, err)
	same, err := StringConcat(2, 16)
	require.NoError(t, err)
	different, err := StringConcat(3, 8)
	require.NoError(t, err)
	for _, bounds := range [][2]int{{1, 2}, {2, 17}, {8, 4}} {
		_, err := StringConcat(bounds[0], bounds[1])
		require.Error(t, err)
	}
	hash := func(point *stringConcat) string {
		hasher := fingerprint.New()
		require.NoError(t, point.Hash(hasher))
		return hasher.Finish()
	}
	require.Equal(t, hash(first), hash(same))
	require.NotEqual(t, hash(first), hash(different))
}

func TestStringConcatYAML(t *testing.T) {
	unmarshal := unmarshalers["string-concat"]
	require.NotNil(t, unmarshal)
	for _, test := range []struct {
		source   string
		min, max int
	}{
		{source: `{}`, min: 2, max: 16},
		{source: `{min-operands: 4, max-operands: 8}`, min: 4, max: 8},
	} {
		var value any
		require.NoError(t, yaml.Unmarshal([]byte(test.source), &value))
		node, err := yaml.ValueToNode(value)
		require.NoError(t, err)
		point, err := unmarshal(gocontext.Background(), node)
		require.NoError(t, err)
		concat := point.(*stringConcat)
		require.Equal(t, test.min, concat.MinOperands)
		require.Equal(t, test.max, concat.MaxOperands)
	}
}

func TestSliceExpressionYAMLAndHash(t *testing.T) {
	unmarshal := unmarshalers["slice-expression"]
	require.NotNil(t, unmarshal)
	for source, expected := range map[string]SliceExpressionOperand{
		`{operand: string}`: SliceExpressionOperandString,
		`{operand: bytes}`:  SliceExpressionOperandBytes,
	} {
		var value any
		require.NoError(t, yaml.Unmarshal([]byte(source), &value))
		node, err := yaml.ValueToNode(value)
		require.NoError(t, err)
		point, err := unmarshal(gocontext.Background(), node)
		require.NoError(t, err)
		require.Equal(t, expected, point.(*sliceExpression).Operand)
	}
	for _, source := range []string{`{}`, `{operand: words}`} {
		var value any
		require.NoError(t, yaml.Unmarshal([]byte(source), &value))
		node, err := yaml.ValueToNode(value)
		require.NoError(t, err)
		_, err = unmarshal(gocontext.Background(), node)
		require.Error(t, err)
	}
	stringHasher, bytesHasher := fingerprint.New(), fingerprint.New()
	require.NoError(t, SliceExpression(SliceExpressionOperandString).Hash(stringHasher))
	require.NoError(t, SliceExpression(SliceExpressionOperandBytes).Hash(bytesHasher))
	require.NotEqual(t, stringHasher.Finish(), bytesHasher.Finish())
}
