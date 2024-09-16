// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package cmd_test

import (
	"bytes"
	"flag"
	"fmt"
	"runtime"
	"testing"

	"github.com/datadog/orchestrion/internal/cmd"
	"github.com/datadog/orchestrion/internal/version"
	"github.com/stretchr/testify/require"
	"github.com/urfave/cli/v2"
)

func TestVersion(t *testing.T) {
	set := flag.NewFlagSet("test", flag.ContinueOnError)
	_ = set.Bool("verbose", false, "")

	t.Run("standard", func(t *testing.T) {
		var output bytes.Buffer
		set := *set
		set.Parse(nil)
		ctx := cli.NewContext(&cli.App{Writer: &output}, &set, nil)

		require.NoError(t, cmd.Version.Action(ctx))
		require.Equal(t, fmt.Sprintf("orchestrion %s\n", version.Tag), output.String())
	})

	t.Run("standard with respawn", func(t *testing.T) {
		var output bytes.Buffer
		t.Setenv("DD_ORCHESTRION_STARTUP_VERSION", "v0.0.0")
		set := *set
		set.Parse(nil)
		ctx := cli.NewContext(&cli.App{Writer: &output}, &set, nil)

		require.NoError(t, cmd.Version.Action(ctx))
		require.Equal(t, fmt.Sprintf("orchestrion %s\n", version.Tag), output.String())
	})

	t.Run("verbose", func(t *testing.T) {
		var output bytes.Buffer
		set := *set
		set.Parse([]string{"-verbose"})
		ctx := cli.NewContext(&cli.App{Writer: &output}, &set, nil)

		require.NoError(t, cmd.Version.Action(ctx))
		require.Equal(t, fmt.Sprintf("orchestrion %s built with %s (%s/%s)\n", version.Tag, runtime.Version(), runtime.GOOS, runtime.GOARCH), output.String())
	})

	t.Run("verbose with respawn", func(t *testing.T) {
		var output bytes.Buffer
		t.Setenv("DD_ORCHESTRION_STARTUP_VERSION", "v0.0.0")
		set := *set
		set.Parse([]string{"-verbose"})
		ctx := cli.NewContext(&cli.App{Writer: &output}, &set, nil)

		require.NoError(t, cmd.Version.Action(ctx))
		require.Equal(t, fmt.Sprintf("orchestrion %s (started as v0.0.0) built with %s (%s/%s)\n", version.Tag, runtime.Version(), runtime.GOOS, runtime.GOARCH), output.String())
	})
}
