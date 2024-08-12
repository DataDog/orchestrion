// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package join

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDirectiveMatch(t *testing.T) {
	dir := Directive("test:directive")

	require.True(t, dir.matches("\t//test:directive"))
	require.True(t, dir.matches("\t//test:directive   "))
	require.True(t, dir.matches("\t//test:directive with:arguments"))

	// Not the same directive at all
	require.False(t, dir.matches("\t//test:different"))
	require.False(t, dir.matches("\t//test:directive2"))
	// Not a directive (space after the //)
	require.False(t, dir.matches("\t// test:directive"))
	// Not a directive (not a single-line comment syntax)
	require.False(t, dir.matches("\t/*test:directive*/"))
}
