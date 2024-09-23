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
	Action: func(*cli.Context) error {
		return pin.PinOrchestrion()
	},
}
