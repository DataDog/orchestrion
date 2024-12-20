// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package cmd

import (
	"fmt"
	"os"

	"github.com/DataDog/orchestrion/internal/log"
	"github.com/DataDog/orchestrion/internal/pin"
	"github.com/DataDog/orchestrion/internal/toolexec"
	"github.com/DataDog/orchestrion/internal/toolexec/aspect"
	"github.com/DataDog/orchestrion/internal/toolexec/proxy"
	"github.com/urfave/cli/v2"
)

var Toolexec = &cli.Command{
	Name:            "toolexec",
	Usage:           "Standard `-toolexec` plugin for the Go toolchain",
	UsageText:       "orchestrion toolexec [tool] [tool args...]",
	Args:            true,
	SkipFlagParsing: true,
	Action: func(c *cli.Context) error {
		proxyCmd, err := proxy.ParseCommand(c.Args().Slice())
		if err != nil {
			return err
		}
		defer proxyCmd.Close()

		if proxyCmd.Type() == proxy.CommandTypeOther {
			// Immediately run the command if it's of the Other type, as we do not do
			// any kind of processing on these...
			return proxy.RunCommand(proxyCmd)
		}

		// Ensure Orchestrion is properly pinned
		pin.AutoPinOrchestrion()

		if proxyCmd.ShowVersion() {
			log.Tracef("Toolexec version command: %#v\n", proxyCmd.Args())
			fullVersion, err := toolexec.ComputeVersion(proxyCmd)
			if err != nil {
				return err
			}
			log.Tracef("Complete version output: %s\n", fullVersion)
			_, err = fmt.Println(fullVersion)
			return err
		}

		log.Tracef("Toolexec original command: %q\n", proxyCmd.Args())
		weaver := aspect.Weaver{ImportPath: os.Getenv("TOOLEXEC_IMPORTPATH")}

		if err := proxy.ProcessCommand(proxyCmd, weaver.OnCompile); err != nil {
			return err
		}
		if err := proxy.ProcessCommand(proxyCmd, weaver.OnCompileMain); err != nil {
			return err
		}
		if err := proxy.ProcessCommand(proxyCmd, weaver.OnLink); err != nil {
			return err
		}

		log.Tracef("Toolexec final command:    %q\n", proxyCmd.Args())
		if err := proxy.RunCommand(proxyCmd); err != nil {
			// Logging as debug, as the error will likely surface back to the user anyway...
			log.Debugf("Proxied command failed: %v\n", err)
			return err
		}
		return nil
	},
}
