// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package main

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/datadog/orchestrion/internal/ensure"
	"github.com/datadog/orchestrion/internal/goenv"
	"github.com/datadog/orchestrion/internal/goproxy"
	"github.com/datadog/orchestrion/internal/injector/builtin"
	"github.com/datadog/orchestrion/internal/jobserver"
	"github.com/datadog/orchestrion/internal/log"
	"github.com/datadog/orchestrion/internal/toolexec"
	"github.com/datadog/orchestrion/internal/toolexec/aspect"
	"github.com/datadog/orchestrion/internal/toolexec/proxy"
	"github.com/datadog/orchestrion/internal/version"
)

func main() {
	defer log.Close()

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
		if startupVersion := ensure.StartupVersion(); startupVersion != version.Tag {
			fmt.Printf("%s (started via %s)\n", version.Tag, startupVersion)
		} else {
			fmt.Println(version.Tag)
		}
		return
	case "pin":
		if err := pinOrchestrion(); err != nil {
			fmt.Fprintf(os.Stderr, "An error occurred: %v\n", err)
			os.Exit(1)
		}
		return
	case "go":
		autoPinOrchestrion()
		if err := goproxy.Run(args, goproxy.WithToolexec(orchestrionBinPath, "toolexec")); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		return
	case "toolexec":
		if len(args) == 0 {
			fmt.Fprintf(os.Stderr, "Error: no arguments provided to `orchestrion toolexec`\n")
			os.Exit(2)
		}

		proxyCmd := proxy.MustParseCommand(args)
		defer proxyCmd.Close()
		switch proxyCmd.Type() {
		case proxy.CommandTypeCompile, proxy.CommandTypeLink:
			// We only do `autoPinOrchestrion` in case the command is a "known" and interesting one. It is
			// otherwise a waste of time and risks failing due to working directory issues. For example,
			// the `asm` commands run with the current working directory set to the source tree of the
			// package, whereas `compile` and `link` run in the directrory where the `go.mod` file is.
			autoPinOrchestrion()
		}

		if proxyCmd.ShowVersion() {
			if proxyCmd.Type() == proxy.CommandTypeOther {
				// proxy.CommandTypeOther commands are not subject to any injection, so it is not useful to
				// cache-bust their output artifacts when orchestrion somehow changes. We just run these
				// as-is.
				proxy.MustRunCommand(proxyCmd)
				return
			}

			log.Tracef("Toolexec version command: %q\n", proxyCmd.Args())
			fullVersion, err := toolexec.ComputeVersion(proxyCmd)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Unable to compute version: %v\n", err)
				os.Exit(1)
			}
			log.Tracef("Complete version output: %s\n", fullVersion)
			fmt.Println(fullVersion)
			return
		}

		log.Tracef("Toolexec original command: %q\n", proxyCmd.Args())
		weaver := aspect.Weaver{ImportPath: os.Getenv("TOOLEXEC_IMPORTPATH")}
		if err := proxy.ProcessCommand(proxyCmd, weaver.OnCompile); err != nil {
			if errors.Is(err, proxy.ErrSkipCommand) {
				log.Infof("SKIP: %q\n", proxyCmd.Args())
				return
			}
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		if err := proxy.ProcessCommand(proxyCmd, weaver.OnCompileMain); err != nil {
			if errors.Is(err, proxy.ErrSkipCommand) {
				log.Infof("SKIP: %q\n", proxyCmd.Args())
				return
			}
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		if err := proxy.ProcessCommand(proxyCmd, weaver.OnLink); err != nil {
			if errors.Is(err, proxy.ErrSkipCommand) {
				log.Infof("SKIP: %q\n", proxyCmd.Args())
				return
			}
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}

		// Spacing so the final command is aligned with the original one...
		log.Tracef("Toolexec final command:    %q\n", proxyCmd.Args())
		proxy.MustRunCommand(proxyCmd)

		return
	case "warmup":
		if goMod, err := goenv.GOMOD(); err == nil && goMod != "" {
			// Ensure Orchestrion is pinned here...
			autoPinOrchestrion()
			log.Tracef("Warming up in the current module (%q)\n", goMod)
		} else {
			tmp, err := os.MkdirTemp("", "orchestrion-warmup-*")
			if err != nil {
				fmt.Fprintf(os.Stderr, "Unable to create temporary directory: %v\n", err)
				os.Exit(1)
			}
			defer os.RemoveAll(tmp)
			log.Tracef("Initializing warm-up module in %q\n", tmp)
			var stderr bytes.Buffer
			cmd := exec.Command("go", "mod", "init", "orchestrion-warmup")
			cmd.Dir = tmp
			cmd.Stderr = &stderr
			if err := cmd.Run(); err != nil {
				fmt.Fprintf(os.Stderr, "Unable to initialize temporary module (%q): %v\n", cmd.Args, err)
				if stderr.Len() > 0 {
					fmt.Fprintf(os.Stderr, "Error output:\n%s\n", stderr.String())
				}
				os.Exit(1)
			}

			log.Tracef("Running 'orchestrion pin' in the temporary module...\n")
			stderr.Reset()
			cmd = exec.Command(orchestrionBinPath, "pin")
			cmd.Dir = tmp
			cmd.Stderr = &stderr
			if err := cmd.Run(); err != nil {
				fmt.Fprintf(os.Stderr, "Unable to install orchestrion in temporary module (%q): %v\n", cmd.Args, err)
				if stderr.Len() > 0 {
					fmt.Fprintf(os.Stderr, "Error output:\n%s\n", stderr.String())
				}
				os.Exit(1)
			}

			log.Tracef("Running `go build -v <modules>` in the temporary module...\n")
			buildArgs := make([]string, 0, 3+len(args)+len(builtin.InjectedPaths))
			buildArgs = append(buildArgs, "go", "build")
			buildArgs = append(buildArgs, args...)
			// All packages we may be instrumenting, plus the standard library.
			buildArgs = append(buildArgs, "std")
			buildArgs = append(buildArgs, builtin.InjectedPaths[:]...)
			cmd = exec.Command(orchestrionBinPath, buildArgs...)
			cmd.Dir = tmp
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr
			if err := cmd.Run(); err != nil {
				fmt.Fprintf(os.Stderr, "Warm-up build failed (%q): %v\n", cmd.Args, err)
				os.Exit(1)
			}
		}
		return
	case "server":
		jobserver.Run(args)
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
		"pin",
		"warmup",
	}
	fmt.Printf("Usage:\n    %s <command> [arguments]\n\n", cmd)
	fmt.Println("Available commands:")
	for _, cmd := range commands {
		fmt.Printf("    %s\n", cmd)
	}
}

var (
	orchestrionBinPath string // The path to the current executable file
)

func init() {
	var err error
	if orchestrionBinPath, err = os.Executable(); err != nil {
		if orchestrionBinPath, err = filepath.Abs(os.Args[0]); err != nil {
			orchestrionBinPath = os.Args[0]
		}
	}
	orchestrionBinPath = filepath.Clean(orchestrionBinPath)
}
