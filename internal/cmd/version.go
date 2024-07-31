// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package cmd

import (
	"fmt"

	"github.com/datadog/orchestrion/internal/ensure"
	"github.com/datadog/orchestrion/internal/version"
	"github.com/urfave/cli/v2"
)

var Version = &cli.Command{
	Name:  "version",
	Usage: "Displays this command's version information",
	Flags: []cli.Flag{
		&cli.BoolFlag{
			Name:    "verbose",
			Aliases: []string{"v"},
			Usage:   "display the version of the orchestrion binary that started this command (if different from the current)",
			Hidden:  true,
		},
	},
	Action: func(c *cli.Context) error {
		if c.Bool("verbose") {
			if startupVersion := ensure.StartupVersion(); startupVersion != version.Tag {
				fmt.Printf("%s (started via %s)\n", version.Tag, startupVersion)
				return nil
			}
		}
		fmt.Println(version.Tag)
		return nil
	},
}
