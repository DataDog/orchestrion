// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package aspect

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/DataDog/orchestrion/internal/toolexec/aspect/linkdeps"
	"github.com/DataDog/orchestrion/internal/toolexec/importcfg"
	"github.com/blakesmith/ar"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestInitialLinkDependenciesRetainParent(t *testing.T) {
	archive := writeLinkDepsArchive(t, "parent.a", "example.com/dependency", linkdeps.ImportDependency)
	reg := importcfg.ImportConfig{PackageFile: map[string]string{"example.com/parent": archive}}

	deps, err := initialLinkDependencies(context.Background(), &reg, "example.com/subject")
	require.NoError(t, err)
	require.Len(t, deps, 1)
	assert.Equal(t, "example.com/dependency", deps[0].path)
	assert.Equal(t, "example.com/parent", deps[0].parent)
	assert.Equal(t, linkdeps.ImportDependency, deps[0].kind)
}

func TestInitialLinkDependenciesClearTestTargetParent(t *testing.T) {
	archive := writeLinkDepsArchive(t, "subject.a", "example.com/dependency", linkdeps.ImportDependency)
	reg := importcfg.ImportConfig{PackageFile: map[string]string{"example.com/subject": archive}}

	deps, err := initialLinkDependencies(context.Background(), &reg, "example.com/subject")
	require.NoError(t, err)
	require.Len(t, deps, 1)
	assert.Empty(t, deps[0].parent)
}

func writeLinkDepsArchive(t *testing.T, name string, dependency string, kind linkdeps.DependencyKind) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	file, err := os.Create(path)
	require.NoError(t, err)
	defer file.Close()

	writer := ar.NewWriter(file)
	require.NoError(t, writer.WriteGlobalHeader())
	var metadata linkdeps.LinkDeps
	metadata.Add(dependency, kind)
	var contents bytes.Buffer
	require.NoError(t, metadata.Write(&contents))
	require.NoError(t, writer.WriteHeader(&ar.Header{Name: linkdeps.Filename, Mode: 0o644, Size: int64(contents.Len())}))
	_, err = writer.Write(contents.Bytes())
	require.NoError(t, err)
	return path
}
