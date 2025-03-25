package join

import (
	"testing"

	"github.com/dave/dst"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"

	"github.com/DataDog/orchestrion/internal/fingerprint"
)

func TestSignatureContains(t *testing.T) {
	tests := []struct {
		name     string
		args     []TypeName
		ret      []TypeName
		funcInfo functionInformation
		want     bool
	}{
		{
			name: "single argument matches",
			args: []TypeName{
				{name: "string"},
			},
			ret: make([]TypeName, 0),
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
			args: make([]TypeName, 0),
			ret: []TypeName{
				{name: "error"},
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
			args: []TypeName{
				{name: "string"},
			},
			ret: make([]TypeName, 0),
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
			args: make([]TypeName, 0),
			ret: []TypeName{
				{name: "error"},
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
			args: []TypeName{
				{name: "string"},
			},
			ret: make([]TypeName, 0),
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
			args: []TypeName{
				{name: "float64"},
			},
			ret: []TypeName{
				{name: "byte"},
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
			args: []TypeName{
				{name: "CustomType", path: "pkg"},
			},
			ret: make([]TypeName, 0),
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
	args := []TypeName{{name: "string"}, {name: "int"}}
	ret := []TypeName{{name: "error"}}

	fo := SignatureContains(args, ret)

	h1 := fingerprint.New()
	err := fo.Hash(h1)
	require.NoError(t, err, "Hash failed")

	// Get the fingerprint string
	fp1 := h1.Finish()

	// Create identical signature and verify hash is the same
	fo2 := SignatureContains(args, ret)
	h2 := fingerprint.New()
	err = fo2.Hash(h2)
	require.NoError(t, err, "Hash failed")

	// Get the fingerprint string
	fp2 := h2.Finish()

	assert.Equal(t, fp1, fp2, "Hash() gave different results for identical signatures")

	// Different arguments should give different hash
	fo3 := SignatureContains([]TypeName{{name: "float64"}}, ret)
	h3 := fingerprint.New()
	err = fo3.Hash(h3)
	require.NoError(t, err, "Hash failed")

	// Get the fingerprint string
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
	assert.Equal(t, "string", signatureContains.Arguments[0].Name(), "First argument should be string")
	assert.Equal(t, "error", signatureContains.Arguments[1].Name(), "Second argument should be error")

	require.Len(t, signatureContains.Results, 1, "Expected 1 result")
	assert.Equal(t, "bool", signatureContains.Results[0].Name(), "Result should be bool")
}
