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
	"path/filepath"
	"runtime/debug"
	"strings"
	"time"

	"github.com/datadog/orchestrion/internal/ensure"
	"github.com/datadog/orchestrion/internal/goflags"
	"github.com/datadog/orchestrion/internal/goproxy"
	"github.com/datadog/orchestrion/internal/injector/builtin"
	"github.com/datadog/orchestrion/internal/toolexec/processors/aspect"
	"github.com/datadog/orchestrion/internal/toolexec/proxy"
	"github.com/datadog/orchestrion/internal/toolexec/utils"
	"github.com/datadog/orchestrion/internal/version"

	"github.com/shirou/gopsutil/v3/process"
	"golang.org/x/mod/semver"
)

// ORCHESTRION_GO_COMMAND_FLAGS is used to pass go command invocation flags
// from one process to another. This is needed to preserve build ids with respect to
// the original go command invocation when invoking go commands on our own
const envGoCommandFlags = "ORCHESTRION_GO_COMMAND_FLAGS"

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
		flags, err := parentGoCommandFlags()
		// TODO: inject in args
		if err == nil {
			os.WriteFile("/tmp/orchestrionlog.txt", []byte(flags.String()), 0o644)
		}
		proxyCmd := proxy.MustParseCommand(args)
		defer proxyCmd.Close()
		if proxyCmd.ShowVersion() {
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
			fmt.Printf("%s:orchestrion@%s,%s\n", strings.TrimSpace(stdout.String()), versionString, builtin.Checksum)
			return
		}

		weaver := aspect.Weaver{ImportPath: os.Getenv("TOOLEXEC_IMPORTPATH")}
		if err := proxy.ProcessCommand(proxyCmd, weaver.OnCompile); err != nil {
			if errors.Is(err, proxy.ErrSkipCommand) {
				log.Printf("SKIP: %q\n", proxyCmd.Args())
				return
			}
			utils.ExitIfError(err)
		}
		if err := proxy.ProcessCommand(proxyCmd, weaver.OnLink); err != nil {
			if errors.Is(err, proxy.ErrSkipCommand) {
				log.Printf("SKIP: %q\n", proxyCmd.Args())
				return
			}
			utils.ExitIfError(err)
		}
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

func commandFlags() (goflags.CommandFlags, error) {
	flagsStr, set := os.LookupEnv(envGoCommandFlags)
	if set {
		return goflags.CommandFlagsFromString(flagsStr), nil
	}

	return parentGoCommandFlags()
}

// parentGoCommandFlags backtracks through the process tree
// to find a parent go command invocation and returns its arguments
func parentGoCommandFlags() (flags goflags.CommandFlags, err error) {
	goBin, err := goproxy.GoBin()
	if err != nil {
		return flags, err
	}

	p, err := process.NewProcess(int32(os.Getpid()))
	if err != nil {
		return flags, err
	}

	// Backtrack through the process stack until we find the parent Go command
	var args []string
	for {
		p, err = p.Parent()
		if err != nil {
			return flags, err
		}
		args, err = p.CmdlineSlice()
		if err != nil {
			return flags, err
		}
		cmd, err := exec.LookPath(args[0])
		if err != nil {
			return flags, err
		}
		// Found the go command process, break out of backtracking
		if cmd == goBin {
			break
		}
	}

	return goflags.ParseCommandFlags(args), nil
}
