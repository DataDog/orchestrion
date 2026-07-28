// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package linkdeps

import (
	"bytes"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDependencyKindsRoundTrip(t *testing.T) {
	var original LinkDeps
	original.Add("example.com/relocation", RelocationDependency)
	original.Add("example.com/import", ImportDependency)

	var encoded bytes.Buffer
	require.NoError(t, original.Write(&encoded))
	decoded, err := Read(&encoded)
	require.NoError(t, err)
	assert.Equal(t, RelocationDependency, decoded.Kind("example.com/relocation"))
	assert.Equal(t, ImportDependency, decoded.Kind("example.com/import"))
}

func TestV1DependenciesAreConservative(t *testing.T) {
	for _, suffix := range []string{"\n", ""} {
		decoded, err := Read(strings.NewReader("#link.deps@v1\nexample.com/legacy" + suffix))
		require.NoError(t, err)
		assert.Equal(t, ImportDependency, decoded.Kind("example.com/legacy"))
	}
}

func TestV1HeaderWithoutNewline(t *testing.T) {
	decoded, err := Read(strings.NewReader("#link.deps@v1"))
	require.NoError(t, err)
	assert.True(t, decoded.Empty())
}

func TestImportDependencyWins(t *testing.T) {
	var deps LinkDeps
	deps.Add("example.com/dependency", RelocationDependency)
	deps.Add("example.com/dependency", ImportDependency)
	deps.Add("example.com/dependency", RelocationDependency)
	assert.Equal(t, ImportDependency, deps.Kind("example.com/dependency"))
}
