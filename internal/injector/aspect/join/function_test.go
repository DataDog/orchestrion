// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package join

import (
	"go/types"
	"testing"

	"github.com/dave/dst"
	"github.com/goccy/go-yaml"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/DataDog/orchestrion/internal/fingerprint"
	aspectcontext "github.com/DataDog/orchestrion/internal/injector/aspect/context"
	"github.com/DataDog/orchestrion/internal/injector/typed"
)

func TestSignatureContains(t *testing.T) {
	tests := []struct {
		name     string
		args     []typed.TypeName
		ret      []typed.TypeName
		funcInfo functionInformation
		want     bool
	}{
		{
			name: "single argument matches",
			args: []typed.TypeName{
				{Name: "string"},
			},
			ret: make([]typed.TypeName, 0),
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
			args: make([]typed.TypeName, 0),
			ret: []typed.TypeName{
				{Name: "error"},
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
			args: []typed.TypeName{
				{Name: "string"},
			},
			ret: make([]typed.TypeName, 0),
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
			args: make([]typed.TypeName, 0),
			ret: []typed.TypeName{
				{Name: "error"},
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
			args: []typed.TypeName{
				{Name: "string"},
			},
			ret: make([]typed.TypeName, 0),
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
			args: []typed.TypeName{
				{Name: "float64"},
			},
			ret: []typed.TypeName{
				{Name: "byte"},
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
			args: []typed.TypeName{
				{Name: "CustomType", ImportPath: "pkg"},
			},
			ret: make([]typed.TypeName, 0),
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
	args := []typed.TypeName{{Name: "string"}, {Name: "int"}}
	ret := []typed.TypeName{{Name: "error"}}

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

	fo3 := SignatureContains([]typed.TypeName{{Name: "float64"}}, ret)
	h3 := fingerprint.New()
	err = fo3.Hash(h3)
	require.NoError(t, err, "Hash failed")

	fp3 := h3.Finish()

	assert.NotEqual(t, fp1, fp3, "Hash() gave same result for different signatures")
}

func TestUnmarshalYAMLReceiver(t *testing.T) {
	tests := []struct {
		name    string
		yaml    string
		want    FunctionOption
		wantErr bool
	}{
		{
			name: "receiver type",
			yaml: `receiver: net/http.Server`,
			want: Receiver(typed.TypeName{
				ImportPath: "net/http",
				Name:       "Server",
			}),
		},
		{
			name: "no receiver",
			yaml: `receiver: false`,
			want: NoReceiver(),
		},
		{
			name: "quoted false is a receiver type",
			yaml: `receiver: "false"`,
			want: Receiver(typed.TypeName{Name: "false"}),
		},
		{
			name: "any receiver",
			yaml: `receiver: true`,
			want: AnyReceiver(),
		},
		{
			name:    "non-string is rejected",
			yaml:    `receiver: 42`,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var option unmarshalFuncDeclOption
			err := yaml.Unmarshal([]byte(tt.yaml), &option)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, option.FunctionOption)
		})
	}
}

func TestHasReceiverMatches(t *testing.T) {
	tests := []struct {
		name            string
		node            dst.Node
		wantNoReceiver  bool
		wantAnyReceiver bool
	}{
		{
			name:            "function declaration",
			node:            &dst.FuncDecl{Name: dst.NewIdent("function")},
			wantNoReceiver:  true,
			wantAnyReceiver: false,
		},
		{
			name: "method declaration",
			node: &dst.FuncDecl{
				Recv: &dst.FieldList{List: []*dst.Field{{Type: dst.NewIdent("receiver")}}},
				Name: dst.NewIdent("method"),
			},
			wantNoReceiver:  false,
			wantAnyReceiver: true,
		},
		{
			name: "pointer method declaration",
			node: &dst.FuncDecl{
				Recv: &dst.FieldList{List: []*dst.Field{{Type: &dst.StarExpr{X: dst.NewIdent("receiver")}}}},
				Name: dst.NewIdent("method"),
			},
			wantNoReceiver:  false,
			wantAnyReceiver: true,
		},
		{
			name:            "function literal",
			node:            &dst.FuncLit{Type: &dst.FuncType{}},
			wantNoReceiver:  true,
			wantAnyReceiver: false,
		},
		{
			name:            "non-function node",
			node:            &dst.GenDecl{},
			wantNoReceiver:  false,
			wantAnyReceiver: false,
		},
	}

	noReceiver := Function(NoReceiver())
	anyReceiver := Function(AnyReceiver())
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := functionTestContext{node: tt.node}
			assert.Equal(t, tt.wantNoReceiver, noReceiver.Matches(ctx))
			assert.Equal(t, tt.wantAnyReceiver, anyReceiver.Matches(ctx))
		})
	}
}

type functionTestContext struct {
	node dst.Node
}

func (functionTestContext) Chain() *aspectcontext.NodeChain     { return nil }
func (ctx functionTestContext) Node() dst.Node                  { return ctx.node }
func (functionTestContext) Parent() aspectcontext.AspectContext { return nil }
func (functionTestContext) Config(string) (string, bool)        { return "", false }
func (functionTestContext) File() *dst.File                     { return nil }
func (functionTestContext) ImportPath() string                  { return "example.com/test" }
func (functionTestContext) Package() string                     { return "test" }
func (functionTestContext) TestMain() bool                      { return false }
func (functionTestContext) Release()                            {}
func (functionTestContext) ResolveType(dst.Expr) types.Type     { return nil }

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
	assert.Equal(t, "string", signatureContains.Arguments[0].Name, "First argument should be string")
	assert.Equal(t, "error", signatureContains.Arguments[1].Name, "Second argument should be error")

	require.Len(t, signatureContains.Results, 1, "Expected 1 result")
	assert.Equal(t, "bool", signatureContains.Results[0].Name, "Result should be bool")
}
