// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package cmd

import (
	"fmt"

	"github.com/DataDog/orchestrion/internal/report"
	"github.com/urfave/cli/v2"
)

var (
	filenameFlag = cli.BoolFlag{
		Name:  "files",
		Usage: "Only show file paths created by orchestrion instead of diff output",
	}

	filterFlag = cli.StringFlag{
		Name:  "filter",
		Usage: "Filter the diff to a regex matched on the package or file paths from the build.",
	}

	packageFlag = cli.BoolFlag{
		Name:  "package",
		Usage: "Print package names instead of printing the diff",
	}

	debugFlag = cli.BoolFlag{
		Name:  "debug",
		Usage: "Also print synthetic and tracer weaved packages",
	}

	Diff = &cli.Command{
		Name:  "diff",
		Usage: "Generates a diff between a nominal and orchestrion-instrumented build using a go work directory that can be obtained running `orchestrion go build -work -a`. This does work with -cover builds.",
		Args:  true,
		Flags: []cli.Flag{
			&filenameFlag,
			&filterFlag,
			&packageFlag,
			&debugFlag,
		},
		Action: func(clictx *cli.Context) (err error) {
			workFolder := clictx.Args().First()
			if workFolder == "" {
				return cli.ShowSubcommandHelp(clictx)
			}

			report, err := report.FromWorkDir(clictx.Context, workFolder)
			if err != nil {
				return cli.Exit(fmt.Sprintf("failed to read work dir: %s (did you forgot the -work flag during build ?)", err), 1)
			}

			if len(report.Files) == 0 {
				return cli.Exit("no files to diff (did you forgot the -a flag during build?)", 1)
			}

			if !clictx.Bool(debugFlag.Name) {
				report = report.WithSpecialCasesFilter()
			}

			if filter := clictx.String(filterFlag.Name); filter != "" {
				report, err = report.WithRegexFilter(filter)
				if err != nil {
					return cli.Exit(fmt.Sprintf("failed to filter files: %s", err), 1)
				}
			}

			if clictx.Bool(packageFlag.Name) {
				for pkg := range report.Packages() {
					fmt.Fprintln(clictx.App.Writer, pkg)
				}
				return nil
			}

			if clictx.Bool(filenameFlag.Name) {
				for _, file := range report.Files {
					fmt.Fprintln(clictx.App.Writer, file.ModifiedPath)
				}
				return nil
			}

			if err := report.Diff(clictx.App.Writer); err != nil {
				return cli.Exit(fmt.Sprintf("failed to generate diff: %s", err), 1)
			}

			return nil
		},
	}
)
