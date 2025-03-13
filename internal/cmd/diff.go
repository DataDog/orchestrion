// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package cmd

import (
	"github.com/DataDog/orchestrion/internal/report"
	"github.com/urfave/cli/v2"
)

var (
	Diff = &cli.Command{
		Name:            "diff",
		Usage:           "Generates a diff between a nominal and orchestrion-instrumented build using a report file created by orchestrion -report {path} go",
		UsageText:       "orchestrion go [go command arguments...]",
		Args:            true,
		SkipFlagParsing: true,
		Action: func(ctx *cli.Context) error {
			if ctx.Args().First() == "" {
				return cli.Exit("missing report file path", 1)
			}
			report, err := report.ParseReport(ctx.Args().First())
			if err != nil {
				return cli.Exit(err, 1)
			}

			return report.Diff(ctx.App.Writer)
		},
	}
)
