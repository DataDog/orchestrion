// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package typed

import (
	"go/token"
	"go/types"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCoreTypePredicates(t *testing.T) {
	pkg := types.NewPackage("example.com/types", "types")
	namedString := types.NewNamed(types.NewTypeName(token.NoPos, pkg, "String", nil), types.Typ[types.String], nil)
	namedByte := types.NewNamed(types.NewTypeName(token.NoPos, pkg, "Byte", nil), types.Typ[types.Byte], nil)
	require.True(t, IsStringCore(types.Typ[types.String]))
	require.True(t, IsStringCore(namedString))
	require.False(t, IsStringCore(types.Typ[types.UntypedString]))
	require.False(t, IsStringCore(nil))
	require.True(t, IsByteSliceCore(types.NewSlice(types.Typ[types.Byte])))
	require.False(t, IsByteSliceCore(types.NewSlice(namedByte)))
	require.False(t, IsByteSliceCore(types.NewArray(types.Typ[types.Byte], 2)))
}

func TestTypeParameterCoreType(t *testing.T) {
	stringTerm := types.NewTerm(true, types.Typ[types.String])
	constraint := types.NewInterfaceType(nil, []types.Type{types.NewUnion([]*types.Term{stringTerm})})
	constraint.Complete()
	parameter := types.NewTypeParam(types.NewTypeName(token.NoPos, nil, "T", nil), constraint)
	require.True(t, IsStringCore(parameter))

	comparableConstraint := types.NewInterfaceType(nil, []types.Type{
		types.NewUnion([]*types.Term{stringTerm}),
		types.Universe.Lookup("comparable").Type(),
	})
	comparableConstraint.Complete()
	comparableParameter := types.NewTypeParam(types.NewTypeName(token.NoPos, nil, "C", nil), comparableConstraint)
	require.True(t, IsStringCore(comparableParameter))

	mixed := types.NewInterfaceType(nil, []types.Type{types.NewUnion([]*types.Term{
		types.NewTerm(true, types.Typ[types.String]),
		types.NewTerm(true, types.Typ[types.Int]),
	})})
	mixed.Complete()
	mixedParameter := types.NewTypeParam(types.NewTypeName(token.NoPos, nil, "M", nil), mixed)
	require.Nil(t, CoreType(mixedParameter))
}
