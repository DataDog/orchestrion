// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package cmd

import (
	"fmt"
	"runtime"

	"github.com/urfave/cli/v2"

	"github.com/DataDog/orchestrion/internal/version"
)

var Version = &cli.Command{
	Name:  "version",
	Usage: "Displays this command's version information",
	Flags: []cli.Flag{
		&cli.BoolFlag{
			Name:   "static",
			Usage:  "only display the static version tag, ignoring build information baked into the binary",
			Hidden: true,
		},
		&cli.BoolFlag{
			Name:    "verbose",
			Aliases: []string{"v"},
			Usage:   "display the version of the orchestrion binary that started this command (if different from the current)",
			Hidden:  true,
		},
	},
	Action: func(c *cli.Context) error {
		tag := version.Tag()
		if c.Bool("static") {
			tag, _ = version.TagInfo()
		}
		if _, err := fmt.Fprintf(c.App.Writer, "orchestrion %s", tag); err != nil {
			return err
		}

		if c.Bool("verbose") {
			if _, err := fmt.Fprintf(c.App.Writer, " built with %s (%s/%s)", runtime.Version(), runtime.GOOS, runtime.GOARCH); err != nil {
				return err
			}
		}

		_, err := fmt.Fprintln(c.App.Writer)
		return err
	},
}
