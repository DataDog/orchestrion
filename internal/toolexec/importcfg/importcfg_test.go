// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package importcfg

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParseWithoutEntries(t *testing.T) {
	// This is the importcfg the go command passes when compiling a package that imports nothing; entries
	// must still be addable to it, e.g. for a dependency injected by an aspect.
	reg, err := parse(strings.NewReader("# import config\n"))
	require.NoError(t, err)
	require.NotNil(t, reg.PackageFile)
	require.NotNil(t, reg.ImportMap)
}
