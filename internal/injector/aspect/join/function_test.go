// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package join

import (
	"testing"

	"github.com/dave/dst"
	"github.com/goccy/go-yaml"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/DataDog/orchestrion/internal/fingerprint"
	"github.com/DataDog/orchestrion/internal/injector/typed"
)

func TestSignatureContains(t *testing.T) {
	tests := []struct {
		name     string
		args     []typed.Type
		ret      []typed.Type
		funcInfo functionInformation
		want     bool
	}{
		{
			name: "single argument matches",
			args: []typed.Type{
				&typed.NamedType{Name: "string"},
			},
			ret: make([]typed.Type, 0),
			funcInfo: functionInformation{
				Type: &dst.FuncType{
					Params: &dst.FieldList{
						List: []*dst.Field{
							{Type: &dst.Ident{Name: "string"}},
							{Type: &dst.Ident{Name: "int"}},
						},
					},
					Results: &dst.FieldList{
						List: make([]*dst.Field, 0),
					},
				},
			},
			want: true,
		},
		{
			name: "single return matches",
			args: make([]typed.Type, 0),
			ret: []typed.Type{
				&typed.NamedType{Name: "error"},
			},
			funcInfo: functionInformation{
				Type: &dst.FuncType{
					Params: &dst.FieldList{
						List: make([]*dst.Field, 0),
					},
					Results: &dst.FieldList{
						List: []*dst.Field{
							{Type: &dst.Ident{Name: "error"}},
						},
					},
				},
			},
			want: true,
		},
		{
			name: "argument in any position matches",
			args: []typed.Type{
				&typed.NamedType{Name: "string"},
			},
			ret: make([]typed.Type, 0),
			funcInfo: functionInformation{
				Type: &dst.FuncType{
					Params: &dst.FieldList{
						List: []*dst.Field{
							{Type: &dst.Ident{Name: "int"}},
							{Type: &dst.Ident{Name: "string"}},
						},
					},
					Results: &dst.FieldList{
						List: make([]*dst.Field, 0),
					},
				},
			},
			want: true,
		},
		{
			name: "return in any position matches",
			args: make([]typed.Type, 0),
			ret: []typed.Type{
				&typed.NamedType{Name: "error"},
			},
			funcInfo: functionInformation{
				Type: &dst.FuncType{
					Params: &dst.FieldList{
						List: make([]*dst.Field, 0),
					},
					Results: &dst.FieldList{
						List: []*dst.Field{
							{Type: &dst.Ident{Name: "string"}},
							{Type: &dst.Ident{Name: "error"}},
						},
					},
				},
			},
			want: true,
		},
		{
			name: "no match for empty fields",
			args: []typed.Type{
				&typed.NamedType{Name: "string"},
			},
			ret: make([]typed.Type, 0),
			funcInfo: functionInformation{
				Type: &dst.FuncType{
					Params:  nil,
					Results: nil,
				},
			},
			want: false,
		},
		{
			name: "no match for different type",
			args: []typed.Type{
				&typed.NamedType{Name: "float64"},
			},
			ret: []typed.Type{
				&typed.NamedType{Name: "byte"},
			},
			funcInfo: functionInformation{
				Type: &dst.FuncType{
					Params: &dst.FieldList{
						List: []*dst.Field{
							{Type: &dst.Ident{Name: "string"}},
						},
					},
					Results: &dst.FieldList{
						List: []*dst.Field{
							{Type: &dst.Ident{Name: "error"}},
						},
					},
				},
			},
			want: false,
		},
		{
			name: "complex type match",
			args: []typed.Type{
				&typed.NamedType{Name: "CustomType", Path: "pkg"},
			},
			ret: make([]typed.Type, 0),
			funcInfo: functionInformation{
				Type: &dst.FuncType{
					Params: &dst.FieldList{
						List: []*dst.Field{
							{
								Type: &dst.SelectorExpr{
									X:   &dst.Ident{Name: "pkg"},
									Sel: &dst.Ident{Name: "CustomType"},
								},
							},
						},
					},
					Results: &dst.FieldList{
						List: make([]*dst.Field, 0),
					},
				},
			},
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fo := SignatureContains(tt.args, tt.ret)
			got := fo.(*signatureContains).evaluate(tt.funcInfo)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestSignatureContainsHash(t *testing.T) {
	args := []typed.Type{&typed.NamedType{Name: "string"}, &typed.NamedType{Name: "int"}}
	ret := []typed.Type{&typed.NamedType{Name: "error"}}

	fo := SignatureContains(args, ret)

	h1 := fingerprint.New()
	err := fo.Hash(h1)
	require.NoError(t, err, "Hash failed")

	fp1 := h1.Finish()

	fo2 := SignatureContains(args, ret)
	h2 := fingerprint.New()
	err = fo2.Hash(h2)
	require.NoError(t, err, "Hash failed")

	fp2 := h2.Finish()

	assert.Equal(t, fp1, fp2, "Hash() gave different results for identical signatures")

	fo3 := SignatureContains([]typed.Type{&typed.NamedType{Name: "float64"}}, ret)
	h3 := fingerprint.New()
	err = fo3.Hash(h3)
	require.NoError(t, err, "Hash failed")

	fp3 := h3.Finish()

	assert.NotEqual(t, fp1, fp3, "Hash() gave same result for different signatures")
}

func TestUnmarshalYAMLSignatureContains(t *testing.T) {
	yamlStr := `
signature-contains:
  args: [string, error]
  returns: [bool]
`

	var option unmarshalFuncDeclOption
	err := yaml.Unmarshal([]byte(yamlStr), &option)
	require.NoError(t, err, "Failed to unmarshal YAML")

	signatureContains, ok := option.FunctionOption.(*signatureContains)
	require.True(t, ok, "Expected *signatureContains, got %T", option.FunctionOption)

	require.Len(t, signatureContains.Arguments, 2, "Expected 2 arguments")
	assert.Equal(t, "string", signatureContains.Arguments[0].UnqualifiedName(), "First argument should be string")
	assert.Equal(t, "error", signatureContains.Arguments[1].UnqualifiedName(), "Second argument should be error")

	require.Len(t, signatureContains.Results, 1, "Expected 1 result")
	assert.Equal(t, "bool", signatureContains.Results[0].UnqualifiedName(), "Result should be bool")
}
