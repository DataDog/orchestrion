// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package goenv

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func Test(t *testing.T) {
	path, err := GoBinPath()
	require.NoError(t, err)
	t.Logf("GOBIN: %s", path)
}
