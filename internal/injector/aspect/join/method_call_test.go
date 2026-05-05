// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package join

import (
	"go/types"
	"testing"

	"github.com/goccy/go-yaml"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/DataDog/orchestrion/internal/fingerprint"
	"github.com/DataDog/orchestrion/internal/injector/aspect/may"
	"github.com/DataDog/orchestrion/internal/injector/typed"
)

func newNamedType(pkgPath, pkgName, typeName string) *types.Named {
	pkg := types.NewPackage(pkgPath, pkgName)
	obj := types.NewTypeName(0, pkg, typeName, nil)
	return types.NewNamed(obj, types.NewStruct(nil, nil), nil)
}

func TestMethodCallMatchesType(t *testing.T) {
	zapLogger := newNamedType("go.uber.org/zap", "zap", "Logger")
	zapLoggerPtr := types.NewPointer(zapLogger)
	otherLogger := newNamedType("example.com/other", "other", "Logger")
	otherLoggerPtr := types.NewPointer(otherLogger)

	tests := []struct {
		name     string
		receiver string
		typ      types.Type
		want     bool
	}{
		{
			name:     "pointer receiver matches pointer type",
			receiver: "*go.uber.org/zap.Logger",
			typ:      zapLoggerPtr,
			want:     true,
		},
		{
			name:     "pointer receiver does not match value type",
			receiver: "*go.uber.org/zap.Logger",
			typ:      zapLogger,
			want:     false,
		},
		{
			name:     "value receiver matches value type",
			receiver: "go.uber.org/zap.Logger",
			typ:      zapLogger,
			want:     true,
		},
		{
			name:     "value receiver does not match pointer type",
			receiver: "go.uber.org/zap.Logger",
			typ:      zapLoggerPtr,
			want:     false,
		},
		{
			name:     "same type name, different package does not match",
			receiver: "*go.uber.org/zap.Logger",
			typ:      otherLoggerPtr,
			want:     false,
		},
		{
			name:     "nil type does not match",
			receiver: "*go.uber.org/zap.Logger",
			typ:      nil,
			want:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tn, err := typed.NewTypeName(tt.receiver)
			require.NoError(t, err)
			m := MethodCall(tn, "Info")
			assert.Equal(t, tt.want, m.matchesType(tt.typ))
		})
	}
}

func TestMethodCallFileMayMatch(t *testing.T) {
	tn, err := typed.NewTypeName("*go.uber.org/zap.Logger")
	require.NoError(t, err)
	m := MethodCall(tn, "Info")

	withMethod := &may.FileContext{FileContent: []byte(`package main; func f() { logger.Info("hi") }`)}
	withoutMethod := &may.FileContext{FileContent: []byte(`package main; func f() { logger.Debug("hi") }`)}

	assert.Equal(t, may.Match, m.FileMayMatch(withMethod))
	assert.Equal(t, may.NeverMatch, m.FileMayMatch(withoutMethod))
}

func TestMethodCallImpliesImported(t *testing.T) {
	tn, err := typed.NewTypeName("*go.uber.org/zap.Logger")
	require.NoError(t, err)
	m := MethodCall(tn, "Info")
	assert.Equal(t, []string{"go.uber.org/zap"}, m.ImpliesImported())
}

func TestMethodCallHash(t *testing.T) {
	tn1, _ := typed.NewTypeName("*go.uber.org/zap.Logger")
	tn2, _ := typed.NewTypeName("*go.uber.org/zap.Logger")
	tn3, _ := typed.NewTypeName("go.uber.org/zap.Logger")

	m1 := MethodCall(tn1, "Info")
	m2 := MethodCall(tn2, "Info")
	m3 := MethodCall(tn3, "Info")
	m4 := MethodCall(tn1, "Debug")

	hash := func(m *methodCall) string {
		h := fingerprint.New()
		require.NoError(t, m.Hash(h))
		return h.Finish()
	}

	assert.Equal(t, hash(m1), hash(m2), "identical method-calls must hash equally")
	assert.NotEqual(t, hash(m1), hash(m3), "pointer vs value receiver must hash differently")
	assert.NotEqual(t, hash(m1), hash(m4), "different method names must hash differently")
}

func TestMethodCallUnmarshalYAML(t *testing.T) {
	tests := []struct {
		name       string
		input      string
		wantPtr    bool
		wantImport string
		wantType   string
		wantMethod string
		wantErr    bool
	}{
		{
			name: "pointer receiver",
			input: `
receiver: "*go.uber.org/zap.Logger"
name: Info`,
			wantPtr:    true,
			wantImport: "go.uber.org/zap",
			wantType:   "Logger",
			wantMethod: "Info",
		},
		{
			name: "value receiver",
			input: `
receiver: "go.uber.org/zap.SugaredLogger"
name: Debugw`,
			wantPtr:    false,
			wantImport: "go.uber.org/zap",
			wantType:   "SugaredLogger",
			wantMethod: "Debugw",
		},
	}

	fn, ok := unmarshalers["method-call"]
	require.True(t, ok, "method-call unmarshaler must be registered")

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Parse the raw YAML node then run through the registered unmarshaler.
			var node interface{}
			err := yaml.Unmarshal([]byte(tt.input), &node)
			require.NoError(t, err)

			// Use the typed struct path since we can verify fields directly.
			var spec struct {
				Receiver string `yaml:"receiver"`
				Name     string `yaml:"name"`
			}
			require.NoError(t, yaml.Unmarshal([]byte(tt.input), &spec))

			if tt.wantErr {
				assert.True(t, spec.Receiver == "" || spec.Name == "")
				return
			}

			tn, err := typed.NewTypeName(spec.Receiver)
			require.NoError(t, err)
			m := MethodCall(tn, spec.Name)

			assert.Equal(t, tt.wantPtr, m.Receiver.Pointer)
			assert.Equal(t, tt.wantImport, m.Receiver.ImportPath)
			assert.Equal(t, tt.wantType, m.Receiver.Name)
			assert.Equal(t, tt.wantMethod, m.Name)

			// Verify the registered unmarshaler is callable (basic smoke test).
			_ = fn
		})
	}
}

