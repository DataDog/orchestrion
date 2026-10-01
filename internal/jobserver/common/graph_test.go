// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package common_test

import (
	"fmt"
	"testing"

	"github.com/DataDog/orchestrion/internal/jobserver/common"
	"github.com/stretchr/testify/require"
)

func Test(t *testing.T) {
	g := common.Graph{}

	require.NoError(t, g.AddEdge("a", "b"))
	require.NoError(t, g.AddEdge("b", "c"))
	require.NoError(t, g.AddEdge("c", "d"))
	// Cycles back to B!
	require.ErrorContains(t, g.AddEdge("d", "b"), "cycle detected: b -> c -> d -> b")
}

func TestCycleThroughOneOfManyChildren(t *testing.T) {
	g := common.Graph{}

	// Only one of x's children has a path back to x, and the others are visited in random order (some
	// before it, some after it); which must not prevent the cycle from being detected.
	require.NoError(t, g.AddEdge("x", "y"))
	require.NoError(t, g.AddEdge("y", "z"))
	for i := range 16 {
		require.NoError(t, g.AddEdge("x", fmt.Sprintf("dead-end-%d", i)))
	}
	for range 10 {
		require.ErrorContains(t, g.AddEdge("z", "x"), "cycle detected: x -> y -> z -> x")
	}
}
