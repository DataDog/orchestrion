// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package gls

import (
	"testing"

	"github.com/stretchr/testify/require"
)

//dd:orchestrion-enabled
const orchestrionEnabled = false

func Test(t *testing.T) {
	expected := "Hello, World!"

	set(expected)
	actual := get()

	if orchestrionEnabled {
		t.Log("Orchestrion IS enabled")
		require.Equal(t, expected, actual)
	} else {
		t.Log("Orchestrion IS NOT enabled")
		require.Nil(t, actual)
	}
}
