// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package join

import (
	gocontext "context"
	"go/types"
	"testing"

	"github.com/goccy/go-yaml"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/DataDog/orchestrion/internal/fingerprint"
	"github.com/DataDog/orchestrion/internal/injector/aspect/may"
	"github.com/DataDog/orchestrion/internal/injector/typed"
)

func newNamedType(pkgPath string, pkgName string, typeName string) *types.Named {
	pkg := types.NewPackage(pkgPath, pkgName)
	obj := types.NewTypeName(0, pkg, typeName, nil)
	return types.NewNamed(obj, types.NewStruct(nil, nil), nil)
}

func TestMethodCallMatchesType(t *testing.T) {
	zapLogger := newNamedType("go.uber.org/zap", "zap", "Logger")
	zapLoggerPtr := types.NewPointer(zapLogger)
	otherLogger := newNamedType("example.com/other", "other", "Logger")
	otherLoggerPtr := types.NewPointer(otherLogger)

	tn, err := typed.NewTypeName("go.uber.org/zap.Logger")
	require.NoError(t, err)

	tests := []struct {
		name  string
		match MethodCallMatch
		typ   types.Type
		want  bool
	}{
		{name: "any: matches pointer", match: MethodCallMatchAny, typ: zapLoggerPtr, want: true},
		{name: "any: matches value", match: MethodCallMatchAny, typ: zapLogger, want: true},
		{name: "pointer-only: matches pointer", match: MethodCallMatchPointerOnly, typ: zapLoggerPtr, want: true},
		{name: "pointer-only: rejects value", match: MethodCallMatchPointerOnly, typ: zapLogger, want: false},
		{name: "value-only: matches value", match: MethodCallMatchValueOnly, typ: zapLogger, want: true},
		{name: "value-only: rejects pointer", match: MethodCallMatchValueOnly, typ: zapLoggerPtr, want: false},
		{name: "any: rejects different package", match: MethodCallMatchAny, typ: otherLoggerPtr, want: false},
		{name: "any: rejects nil", match: MethodCallMatchAny, typ: nil, want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := MethodCall(tn, "Info", tt.match)
			assert.Equal(t, tt.want, m.matchesType(tt.typ))
		})
	}
}

func TestMethodCallPackageMayMatch(t *testing.T) {
	tn, err := typed.NewTypeName("go.uber.org/zap.Logger")
	require.NoError(t, err)
	m := MethodCall(tn, "Info", MethodCallMatchAny)

	importing := &may.PackageContext{ImportMap: map[string]string{"go.uber.org/zap": "zap.a"}}
	notImporting := &may.PackageContext{ImportMap: map[string]string{"example.com/other": "other.a"}}

	assert.Equal(t, may.Match, m.PackageMayMatch(importing))
	assert.Equal(t, may.NeverMatch, m.PackageMayMatch(notImporting))
}

func TestMethodCallFileMayMatch(t *testing.T) {
	tn, err := typed.NewTypeName("go.uber.org/zap.Logger")
	require.NoError(t, err)
	m := MethodCall(tn, "Info", MethodCallMatchAny)

	withMethod := &may.FileContext{FileContent: []byte(`package main; func f() { logger.Info("hi") }`)}
	withoutMethod := &may.FileContext{FileContent: []byte(`package main; func f() { logger.Debug("hi") }`)}

	assert.Equal(t, may.Match, m.FileMayMatch(withMethod))
	assert.Equal(t, may.NeverMatch, m.FileMayMatch(withoutMethod))
}

func TestMethodCallImpliesImported(t *testing.T) {
	tn, err := typed.NewTypeName("go.uber.org/zap.Logger")
	require.NoError(t, err)
	m := MethodCall(tn, "Info", MethodCallMatchAny)
	assert.Equal(t, []string{"go.uber.org/zap"}, m.ImpliesImported())
}

func TestMethodCallHash(t *testing.T) {
	tn1, _ := typed.NewTypeName("go.uber.org/zap.Logger")
	tn2, _ := typed.NewTypeName("go.uber.org/zap.Logger")
	tn3, _ := typed.NewTypeName("example.com/other.Logger")

	m1 := MethodCall(tn1, "Info", MethodCallMatchAny)
	m2 := MethodCall(tn2, "Info", MethodCallMatchAny)
	m3 := MethodCall(tn3, "Info", MethodCallMatchAny)
	m4 := MethodCall(tn1, "Debug", MethodCallMatchAny)
	m5 := MethodCall(tn1, "Info", MethodCallMatchPointerOnly)

	hash := func(m *methodCall) string {
		h := fingerprint.New()
		require.NoError(t, m.Hash(h))
		return h.Finish()
	}

	assert.Equal(t, hash(m1), hash(m2), "identical method-calls must hash equally")
	assert.NotEqual(t, hash(m1), hash(m3), "different receiver packages must hash differently")
	assert.NotEqual(t, hash(m1), hash(m4), "different method names must hash differently")
	assert.NotEqual(t, hash(m1), hash(m5), "different match modes must hash differently")
}

func TestMethodCallUnmarshalYAML(t *testing.T) {
	tests := []struct {
		name       string
		yaml       string
		wantImport string
		wantType   string
		wantMethod string
		wantMatch  MethodCallMatch
		wantErr    bool
	}{
		{
			name: "defaults to any",
			yaml: `method-call:
  receiver: "go.uber.org/zap.Logger"
  name: Info`,
			wantImport: "go.uber.org/zap",
			wantType:   "Logger",
			wantMethod: "Info",
			wantMatch:  MethodCallMatchAny,
		},
		{
			name: "pointer-only",
			yaml: `method-call:
  receiver: "go.uber.org/zap.Logger"
  name: Info
  match: pointer-only`,
			wantImport: "go.uber.org/zap",
			wantType:   "Logger",
			wantMethod: "Info",
			wantMatch:  MethodCallMatchPointerOnly,
		},
		{
			name: "value-only",
			yaml: `method-call:
  receiver: "go.uber.org/zap.SugaredLogger"
  name: Debugw
  match: value-only`,
			wantImport: "go.uber.org/zap",
			wantType:   "SugaredLogger",
			wantMethod: "Debugw",
			wantMatch:  MethodCallMatchValueOnly,
		},
		{
			name: "pointer sigil in receiver is rejected",
			yaml: `method-call:
  receiver: "*go.uber.org/zap.Logger"
  name: Info`,
			wantErr: true,
		},
		{
			name: "missing receiver is rejected",
			yaml: `method-call:
  name: Info`,
			wantErr: true,
		},
		{
			name: "missing name is rejected",
			yaml: `method-call:
  receiver: "go.uber.org/zap.Logger"`,
			wantErr: true,
		},
	}

	fn, ok := unmarshalers["method-call"]
	require.True(t, ok, "method-call unmarshaler must be registered")

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var data map[string]any
			err := yaml.Unmarshal([]byte(tt.yaml), &data)
			require.NoError(t, err)

			node, err := yaml.ValueToNode(data["method-call"])
			require.NoError(t, err)

			result, err := fn(gocontext.Background(), node)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)

			m, ok := result.(*methodCall)
			require.True(t, ok)
			assert.Equal(t, tt.wantImport, m.Receiver.ImportPath)
			assert.Equal(t, tt.wantType, m.Receiver.Name)
			assert.Equal(t, tt.wantMethod, m.Name)
			assert.Equal(t, tt.wantMatch, m.Match)
		})
	}
}
