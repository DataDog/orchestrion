// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package cmd

import (
	"github.com/DataDog/orchestrion/internal/injector/config"
	"github.com/DataDog/orchestrion/internal/pin"
	"github.com/urfave/cli/v2"
)

var Pin = &cli.Command{
	Name:  "pin",
	Usage: "Registers orchestrion in your project's `go.mod` file",
	Flags: []cli.Flag{
		&cli.BoolFlag{
			Name:  "generate",
			Usage: "Add a //go:generate directive to " + config.FilenameOrchestrionToolGo + " to facilitate automated upkeep of its contents.",
			Value: true,
		},
		&cli.BoolFlag{
			Name:  "prune",
			Usage: "Remove imports from " + config.FilenameOrchestrionToolGo + " that do not contain a valid " + config.FilenameOrchestrionYML + " file declaring integrations.",
			Value: true,
		},
		&cli.BoolFlag{
			Name:  "validate",
			Usage: "Validate all " + config.FilenameOrchestrionYML + " files in the project.",
			Value: false,
		},
	},
	Action: func(ctx *cli.Context) error {
		return pin.PinOrchestrion(ctx.Context, pin.Options{
			Writer:     ctx.App.Writer,
			ErrWriter:  ctx.App.ErrWriter,
			Validate:   ctx.Bool("validate"),
			NoGenerate: !ctx.Bool("generate"),
			NoPrune:    !ctx.Bool("prune"),
		})
	},
}
