// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package main

import (
	"bytes"
	"crypto/sha512"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"runtime/debug"
	"strings"

	"github.com/datadog/orchestrion/internal/ensure"
	"github.com/datadog/orchestrion/internal/goproxy"
	"github.com/datadog/orchestrion/internal/injector/builtin"
	"github.com/datadog/orchestrion/internal/log"
	"github.com/datadog/orchestrion/internal/toolexec/aspect"
	"github.com/datadog/orchestrion/internal/toolexec/proxy"
	"github.com/datadog/orchestrion/internal/toolexec/utils"
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

			versionString := bytes.NewBufferString(version.Tag)
			if bi, ok := debug.ReadBuildInfo(); ok {
				var vcsModified bool
				for _, setting := range bi.Settings {
					if setting.Key == "vcs.modified" {
						vcsModified = setting.Value == "true"
						break
					}
				}

				if vcsModified || bi.Main.Version == "(devel)" {
					// If this binary was built with `go build`, it may have VCS information indicating the
					// working directory was dirty (vcsModified). If it was produced with `go run`, it won't
					// have VCS information, but the version may be `(devel)`, indicating it was built from a
					// development branch. In either case, we add a checksum of the current binary to the
					// version string so that development iteration builds aren't frustrated by GOCACHE.
					// We would have wanted to use `bi.Main.Sum` and `bi.Deps.*.Sum` here instead, but the go
					// toolchain does not produce `bi.Main.Sum`, which prevents detecting changes in the main
					// module itself.
					log.Tracef("Detected this build is from a dev tree: vcs.modified=%v; main.Version=%s\n", vcsModified, bi.Main.Version)

					// Determine the current executable path. The file may not be the same as what is running
					// in the current process (it might have been overwritten since), but this is an
					// acceptable approximation.
					exe, err := os.Executable()
					if err != nil {
						// If os.Executable fails, we fall back to os.Args[0].
						log.Debugf("When determining executable path: %v\n", err)
						exe = os.Args[0]
					}

					// We try to open the executable. If that fails, we won't be able to hash it, but we'll
					// ignore this error. The consequence is that GOCACHE entries may be re-used when they
					// shouldn't; which is only a problem on dev iteration. On Windows specifically, this may
					// always fail due to being unable to open a running executable for reading.
					if file, err := os.Open(exe); err == nil {
						sha := sha512.New512_224()
						var buffer [4_096]byte
						if _, err := io.CopyBuffer(sha, file, buffer[:]); err == nil {
							var buf [sha512.Size224]byte
							fmt.Fprintf(versionString, "+%02x", sha.Sum(buf[:0]))
						} else {
							log.Debugf("When hashing executable file: %v\n", err)
						}
					} else {
						log.Debugf("When opening executable file for hashing: %v\n", err)
					}
				}
			}
			log.Tracef("Appending orchestrion information to output: orchestrion@%s,%s\n", versionString.String(), builtin.Checksum)
			fmt.Printf("%s:orchestrion@%s,%s\n", strings.TrimSpace(stdout.String()), versionString.String(), builtin.Checksum)
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
