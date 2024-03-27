// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package main

import (
	"errors"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path"
	"strings"

	"github.com/datadog/orchestrion/internal/injector/builtin"
	"github.com/datadog/orchestrion/internal/version"

	"github.com/datadog/orchestrion/internal/goproxy"
	"github.com/datadog/orchestrion/internal/toolexec/processors"
	"github.com/datadog/orchestrion/internal/toolexec/proxy"
	"github.com/datadog/orchestrion/internal/toolexec/utils"
)

func main() {
	if len(os.Args) < 2 {
		printUsage(os.Args[0])
		return
	}
	cmd := os.Args[1]
	args := os.Args[2:]

	switch cmd {
	case "help":
		printUsage(os.Args[0])
	case "version":
		fmt.Println("orchestrion", version.Tag)
	case "go":
		orchestrion, err := os.Executable()
		if err != nil {
			log.Printf("Error resolving executable path: %v\n", err)
			orchestrion = os.Args[0]
		}
		orchestrion = path.Clean(orchestrion)
		err = goproxy.Run(
			args,
			// goproxy.WithForceBuild(),
			// goproxy.WithDifferentCache(),
			goproxy.WithToolexec([]string{orchestrion, "toolexec"}),
		)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v", err)
			os.Exit(1)
		}
	case "toolexec":
		proxyCmd := proxy.MustParseCommand(args)
		defer func() {
			if err := proxyCmd.Close(); err != nil {
				log.Printf("Error while closing command: %v\n", err)
			}
		}()

		if isVersion, _ := proxyCmd.IsVersion(); isVersion {
			// -V=full is used by the go toolchain to invalidate cache entries when the toolchain changes.
			// We leverage this mechanism to obliterate cache entries if the version of orchestrion
			// changes and/or the built-in rules change.
			cmd := exec.Command(proxyCmd.Args()[0], proxyCmd.Args()[1:]...)
			stdout := &strings.Builder{}
			cmd.Stdout = stdout
			cmd.Stderr = os.Stderr
			utils.ExitIfError(cmd.Run())
			fmt.Printf("%s:orchestrion@%s+%s\n", strings.TrimSpace(stdout.String()), version.Tag, builtin.Checksum)
			return
		}

		if err := proxy.ProcessAllCommands(proxyCmd, &processors.AspectWeaver{}); err != nil {
			if errors.Is(err, proxy.ErrSkipCommand) {
				log.Printf("Skipping requested for %q\n", proxyCmd.Args())
				return
			}
			utils.ExitIfError(err)
		}

		log.Printf("Running possibly modified command %q\n", proxyCmd.Args())
		utils.ExitIfError(proxy.RunCommand(proxyCmd))
		log.Println("Done!")
	default:
		fmt.Fprintf(os.Stderr, "Unknown command '%s'\n\n", cmd)
		printUsage(os.Args[0])
	}
}

func printUsage(cmd string) {
	commands := []string{
		"go",
		"help",
		"toolexec",
		"version",
	}
	fmt.Printf("Usage:\n    %s <command> [arguments]\n\n", cmd)
	fmt.Println("Available commands:")
	for _, cmd := range commands {
		fmt.Printf("    %s\n", cmd)
	}
	fmt.Printf("\nFor more information, run %s help <command>\n", cmd)
}
