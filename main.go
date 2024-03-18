// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package main

import (
	"fmt"
	"os"

	"github.com/datadog/orchestrion/internal/version"

	"github.com/datadog/orchestrion/internal/goproxy"
	"github.com/datadog/orchestrion/internal/toolexec/proxy"
)

func main() {
	if len(os.Args) < 2 {
		printUsage(os.Args[0])
		return
	}
	cmd := os.Args[1]
	args := make([]string, len(os.Args)-2)
	copy(args, os.Args[2:])

	switch cmd {
	case "help":
		printUsage(os.Args[0])
		return
	case "version":
		fmt.Println(version.Tag)
		return
	case "go":
		err := goproxy.Run(args, goproxy.WithForceBuild(), goproxy.WithDifferentCache())
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v", err)
			os.Exit(1)
		}
		return
	case "toolexec":
		proxyCmd := proxy.MustParseCommand(args)
		proxy.MustRunCommand(proxyCmd)
		return
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
