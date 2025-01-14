// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package cmd

import (
	"fmt"
	"os"

	"github.com/DataDog/orchestrion/internal/pin"
	"github.com/DataDog/orchestrion/internal/toolexec"
	"github.com/DataDog/orchestrion/internal/toolexec/aspect"
	"github.com/DataDog/orchestrion/internal/toolexec/proxy"
	"github.com/rs/zerolog"
	"github.com/urfave/cli/v2"
)

var Toolexec = &cli.Command{
	Name:            "toolexec",
	Usage:           "Standard `-toolexec` plugin for the Go toolchain",
	UsageText:       "orchestrion toolexec [tool] [tool args...]",
	Args:            true,
	SkipFlagParsing: true,
	Action: func(ctx *cli.Context) error {
		log := zerolog.Ctx(ctx.Context)

		proxyCmd, err := proxy.ParseCommand(ctx.Args().Slice())
		if err != nil {
			return err
		}
		defer proxyCmd.Close(ctx.Context)

		if proxyCmd.Type() == proxy.CommandTypeOther {
			// Immediately run the command if it's of the Other type, as we do not do
			// any kind of processing on these...
			return proxy.RunCommand(proxyCmd)
		}

		// Ensure Orchestrion is properly pinned
		pin.AutoPinOrchestrion(ctx.Context)

		if proxyCmd.ShowVersion() {
			log.Trace().Strs("command", proxyCmd.Args()).Msg("Toolexec version command")
			fullVersion, err := toolexec.ComputeVersion(ctx.Context, proxyCmd)
			if err != nil {
				return err
			}
			log.Trace().Str("version", fullVersion).Msg("Complete version output")
			_, err = fmt.Println(fullVersion)
			return err
		}

		log.Trace().Strs("command", proxyCmd.Args()).Msg("Toolexec original command")
		weaver := aspect.Weaver{ImportPath: os.Getenv("TOOLEXEC_IMPORTPATH")}

		if err := proxy.ProcessCommand(ctx.Context, proxyCmd, weaver.OnCompile); err != nil {
			return err
		}
		if err := proxy.ProcessCommand(ctx.Context, proxyCmd, weaver.OnCompileMain); err != nil {
			return err
		}
		if err := proxy.ProcessCommand(ctx.Context, proxyCmd, weaver.OnLink); err != nil {
			return err
		}

		log.Trace().Strs("command", proxyCmd.Args()).Msg("Toolexec final command")
		if err := proxy.RunCommand(proxyCmd); err != nil {
			// Logging as debug, as the error will likely surface back to the user anyway...
			log.Debug().Err(err).Msg("Proxied command failed")
			return err
		}
		return nil
	},
}
