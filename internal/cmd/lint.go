// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package cmd

import (
	"fmt"
	"os"
	"slices"
	"strings"
	"text/template"

	"github.com/DataDog/dd-trace-go/v2/ddtrace/tracer"
	"github.com/polyfloyd/go-errorlint/errorlint"
	"github.com/urfave/cli/v2"
	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/multichecker"
)

var Lint = &cli.Command{
	Name:            "lint",
	Usage:           "Run selected static analysis checks on Go code for Orchestrion to work better for certain features.",
	UsageText:       "orchestrion lint [lint arguments...]",
	Args:            true,
	SkipFlagParsing: true,
	Action: func(clictx *cli.Context) (err error) {
		span, _ := tracer.StartSpanFromContext(clictx.Context, "lint",
			tracer.ResourceName(strings.Join(clictx.Args().Slice(), " ")),
		)
		defer func() { span.Finish(tracer.WithError(err)) }()

		// Check if help was requested and print Orchestrion-style header.
		args := clictx.Args().Slice()
		if slices.Contains(args, "-help") || slices.Contains(args, "--help") || slices.Contains(args, "-h") {
			tmpl := template.Must(template.New("help").Parse(cli.CommandHelpTemplate))
			if err := tmpl.Execute(os.Stdout, clictx.Command); err != nil {
				//nolint:errcheck
				fmt.Fprintf(clictx.App.Writer, "NAME:\n   orchestrion lint - %s\n\n", clictx.Command.Usage)
				fmt.Fprintf(clictx.App.Writer, "USAGE:\n   %s\n\n", clictx.Command.UsageText)
				fmt.Fprintln(clictx.App.Writer)
			}
		}

		// Set up os.Args to include the lint subcommand args.
		// Replace "orchestrion lint" with "orchestrion-lint",
		// so multichecker sees proper args
		args = append([]string{"orchestrion-lint"}, args...)
		os.Args = args

		// Run multichecker. This will take over with its own flags.
		analyzers := []*analysis.Analyzer{
			errorlint.NewAnalyzer(
				errorlint.WithComparison(true),
				errorlint.WithAsserts(true),
			),
		}
		multichecker.Main(analyzers...)

		return nil
	},
}
