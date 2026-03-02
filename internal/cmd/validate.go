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
	"github.com/DataDog/orchestrion/internal/validate"
	"github.com/urfave/cli/v2"
)

var Validate = &cli.Command{
	Name:            "validate",
	Usage:           "validate Orchestrion configuration",
	UsageText:       "orchestrion validate [validate arguments...]",
	Args:            true,
	SkipFlagParsing: true,
	Action: func(clictx *cli.Context) (err error) {
		span, ctx := tracer.StartSpanFromContext(clictx.Context, "validate",
			tracer.ResourceName(strings.Join(clictx.Args().Slice(), " ")),
		)
		defer func() { span.Finish(tracer.WithError(err)) }()

		// Check if help was requested and print Orchestrion-style header.
		args := clictx.Args().Slice()
		if slices.Contains(args, "-help") || slices.Contains(args, "--help") || slices.Contains(args, "-h") {
			tmpl := template.Must(template.New("help").Parse(cli.CommandHelpTemplate))
			if err := tmpl.Execute(os.Stdout, clictx.Command); err != nil {
				//nolint:errcheck
				fmt.Fprintf(clictx.App.Writer, "NAME:\n   orchestrion validate - %s\n\n", clictx.Command.Usage)
				fmt.Fprintf(clictx.App.Writer, "USAGE:\n   %s\n\n", clictx.Command.UsageText)
				fmt.Fprintln(clictx.App.Writer)
			}
		}

		return validate.Validate(ctx, args)
	},
}
