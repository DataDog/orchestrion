// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package aspect

import (
	"testing"

	"github.com/DataDog/orchestrion/internal/jobserver/pkgs"
	"github.com/DataDog/orchestrion/internal/toolexec/importcfg"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMergeResolvedArchives(t *testing.T) {
	const target = "example.com/subject"
	reg := importcfg.ImportConfig{PackageFile: map[string]string{
		target:               "/authoritative-subject.a",
		"example.com/shared": "/outer-shared.a",
	}}
	updates, err := mergeResolvedArchives(&reg, pkgs.ResolveResponse{
		"example.com/shared":  {ExportFile: "/resolver-shared.a"},
		"example.com/missing": {ExportFile: "/missing.a"},
		"example.com/variant": {ExportFile: "/variant.a", ForTest: target},
	}, target)
	require.NoError(t, err)
	assert.Equal(t, "/outer-shared.a", reg.PackageFile["example.com/shared"])
	assert.Equal(t, "/missing.a", reg.PackageFile["example.com/missing"])
	assert.Equal(t, "/variant.a", reg.PackageFile["example.com/variant"])
	assert.Equal(t, "/authoritative-subject.a", reg.PackageFile[target])
	assert.Equal(t, map[string]string{
		"example.com/missing": "/missing.a",
		"example.com/variant": "/variant.a",
	}, updates)
}

func TestRejectSyntheticVariantDependency(t *testing.T) {
	const target = "example.com/subject"
	archives := pkgs.ResolveResponse{
		"example.com/middle": {ExportFile: "/middle.a", ForTest: target},
	}
	require.Error(t, rejectSyntheticVariantDependency("example.com/root", "example.com/middle", target, true, archives))
	require.NoError(t, rejectSyntheticVariantDependency("example.com/root", "example.com/middle", target, false, archives))
	require.NoError(t, rejectSyntheticVariantDependency("", "example.com/middle", target, true, archives))
}

func TestMergeResolvedArchivesRejectsWrongVariantTarget(t *testing.T) {
	reg := importcfg.ImportConfig{PackageFile: make(map[string]string)}
	_, err := mergeResolvedArchives(&reg, pkgs.ResolveResponse{
		"example.com/variant": {ExportFile: "/variant.a", ForTest: "example.com/other"},
	}, "example.com/subject")
	require.Error(t, err)
}
