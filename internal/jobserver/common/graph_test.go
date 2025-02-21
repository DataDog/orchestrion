// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package common_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/DataDog/orchestrion/internal/jobserver/common"
)

func Test(t *testing.T) {
	g := common.Graph{}

	require.NoError(t, g.AddEdge("a", "b"))
	require.NoError(t, g.AddEdge("b", "c"))
	require.NoError(t, g.AddEdge("c", "d"))
	// Cycles back to B!
	require.ErrorContains(t, g.AddEdge("d", "b"), "cycle detected: b -> c -> d -> b")
}
