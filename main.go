// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package main

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"runtime/debug"
	"strings"
	"time"

	"github.com/datadog/orchestrion/internal/ensure"
	"github.com/datadog/orchestrion/internal/goproxy"
	"github.com/datadog/orchestrion/internal/injector/builtin"
	"github.com/datadog/orchestrion/internal/log"
	"github.com/datadog/orchestrion/internal/toolexec/aspect"
	"github.com/datadog/orchestrion/internal/toolexec/proxy"
	"github.com/datadog/orchestrion/internal/toolexec/utils"
	"github.com/datadog/orchestrion/internal/version"
	"golang.org/x/mod/semver"
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
		fmt.Println(version.Tag)
		return
	case "go":
		// Ensure we're using the correct version of the tooling...
		if err := ensure.RequiredVersion(); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to detect required version of orchestrion from go.mod: %v\n", err)
			os.Exit(125)
		}

		// Process!
		orchestrion, err := os.Executable()
		if err != nil {
			if orchestrion, err = filepath.Abs(os.Args[0]); err != nil {
				orchestrion = os.Args[0]
			}
		}

		if err := goproxy.Run(args, goproxy.WithToolexec(path.Clean(orchestrion), "toolexec")); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v", err)
			os.Exit(1)
		}
		return
	case "toolexec":
		proxyCmd := proxy.MustParseCommand(args)
		defer proxyCmd.Close()

		if proxyCmd.ShowVersion() {
			log.Tracef("Toolexec version command: %q\n", proxyCmd.Args())

			stdout := strings.Builder{}
			proxy.MustRunCommand(proxyCmd, func(cmd *exec.Cmd) { cmd.Stdout = &stdout })

			versionString := version.Tag
			if strings.HasPrefix(semver.Prerelease(version.Tag), "-dev") {
				if bi, ok := debug.ReadBuildInfo(); ok {
					var revision, vcsTime, modified string
					for _, setting := range bi.Settings {
						switch setting.Key {
						case "vcs.revision":
							revision = setting.Value
						case "vcs.time":
							vcsTime = setting.Value
						case "vcs.modified":
							modified = setting.Value
						}
					}
					const compactTimeFormat = "20060102T150405Z0700"
					switch modified {
					case "": // No VCS information
						versionString = fmt.Sprintf("%s+DEVEL-%s", versionString, time.Now().Format(compactTimeFormat))
					case "true":
						parsed, err := time.Parse(time.RFC3339, vcsTime)
						if err != nil {
							panic(err)
						}
						versionString = fmt.Sprintf("%s+%s-DIRTY-%s", versionString, revision, parsed.Format(compactTimeFormat))
					default:
						versionString = fmt.Sprintf("%s+%s", versionString, revision)
					}
				}
			}
			log.Tracef("Appending orchestrion information to otuput: orchestrion@%s,%s\n", versionString, builtin.Checksum)
			fmt.Printf("%s:orchestrion@%s,%s\n", strings.TrimSpace(stdout.String()), versionString, builtin.Checksum)
			return
		}

		log.Tracef("Toolexec original command: %q\n", proxyCmd.Args())
		weaver := aspect.Weaver{ImportPath: os.Getenv("TOOLEXEC_IMPORTPATH")}
		if err := proxy.ProcessCommand(proxyCmd, weaver.OnCompile); err != nil {
			if errors.Is(err, proxy.ErrSkipCommand) {
				log.Infof("SKIP: %q\n", proxyCmd.Args())
				return
			}
			utils.ExitIfError(err)
		}
		if err := proxy.ProcessCommand(proxyCmd, weaver.OnLink); err != nil {
			if errors.Is(err, proxy.ErrSkipCommand) {
				log.Infof("SKIP: %q\n", proxyCmd.Args())
				return
			}
			utils.ExitIfError(err)
		}

		// Spacing so the final command is aligned with the original one...
		log.Tracef("Toolexec final command:    %q\n", proxyCmd.Args())
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
