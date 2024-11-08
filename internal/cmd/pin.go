// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package cmd

import (
	"github.com/DataDog/orchestrion/internal/pin"
	"github.com/urfave/cli/v2"
)

var Pin = &cli.Command{
	Name:  "pin",
	Usage: "Registers orchestrion in your project's `go.mod` file",
	Flags: []cli.Flag{
		&cli.BoolFlag{
			Name:  "generate",
			Usage: "Add a //go:generate directive to `orchestrion.tool.go` to facilitate automated upkeep of its contents.",
			Value: true,
		},
		&cli.BoolFlag{
			Name:  "prune",
			Usage: "Remove imports from `orchestrion.tool.go` that do not contain a valid `orchestrion.yml` file declaring integrations.",
			Value: true,
		},
	},
	Action: func(ctx *cli.Context) error {
		return pin.PinOrchestrion(pin.Options{
			Writer:     ctx.App.Writer,
			ErrWriter:  ctx.App.ErrWriter,
			NoGenerate: !ctx.Bool("generate"),
			NoPrune:    !ctx.Bool("prune"),
		})
	},
}
