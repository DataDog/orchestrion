// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package importcfg

import (
	"bufio"
	"errors"
	"io"
	"strings"
	"testing"
	"testing/iotest"

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

func TestParseReturnsReadError(t *testing.T) {
	// A read error part way through must be reported, not returned as a
	// truncated, apparently valid ImportConfig.
	errRead := errors.New("read failed")
	r := io.MultiReader(strings.NewReader("packagefile fmt=/tmp/fmt.a\n"), iotest.ErrReader(errRead))
	_, err := parse(r)
	require.ErrorIs(t, err, errRead)
}

func TestParseReturnsTooLongLineError(t *testing.T) {
	// A line longer than the scanner buffer stops the scan, so the error must
	// surface instead of a truncated ImportConfig.
	line := "packagefile fmt=" + strings.Repeat("a", bufio.MaxScanTokenSize+1)
	_, err := parse(strings.NewReader(line + "\n"))
	require.ErrorIs(t, err, bufio.ErrTooLong)
}
