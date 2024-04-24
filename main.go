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
	"github.com/dave/jennifer/jen"
	"github.com/fatih/color"
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
		if !requiredVersionOK {
			warn := color.New(color.BgYellow, color.FgBlack)
			warn.Fprintln(os.Stderr, "╭────────────────────────────────────────────────────────────────────────────────────╮")
			warn.Fprintln(os.Stderr, "│ Warning: github.com/datadog/orchestrion does not appear to be listed in the go.mod │")
			warn.Fprintln(os.Stderr, "│ file. Tracking orchestrion in go.mod ensures consistent, reproductible builds.     │")
			warn.Fprintln(os.Stderr, "│ Run `orchestrion pin` to automatically add orchestrion to your go.mod file.        │")
			warn.Fprintln(os.Stderr, "╰────────────────────────────────────────────────────────────────────────────────────╯")
		}

		if startupVersion := ensure.StartupVersion(); startupVersion != version.Tag {
			fmt.Printf("%s (started via %s)\n", version.Tag, startupVersion)
		} else {
			fmt.Println(version.Tag)
		}
		return
	case "pin":
		if requiredVersionOK {
			fmt.Fprintf(os.Stderr, "Orchestrion is already tracked in the go.mod file. Nothing to do!\n")
			return
		}

		const orchestrionModule = "github.com/datadog/orchestrion"
		func() {
			tools := jen.NewFile("tools")
			tools.HeaderComment("//go:build tools")
			tools.Anon(orchestrionModule)

			file, err := os.OpenFile("orchestrion.tool.go", os.O_CREATE|os.O_WRONLY|os.O_EXCL, 0o644)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Unable to create tools.go file: %v\n", err)
				os.Exit(1)
			}
			defer file.Close()
			if err := tools.Render(file); err != nil {
				// Try to remove the file (it likely contains garbage, if anything...). Ignore errors here.
				_ = file.Close() // Close before attempting to remove
				_ = os.Remove("orchestrion.tool.go")

				fmt.Fprintf(os.Stderr, "Unable to generate tools.go source code: %v\n", err)
				os.Exit(1)
			}
		}()

		if err := exec.Command("go", "get", fmt.Sprintf("%s@%s", orchestrionModule, version.Tag)).Run(); err != nil {
			fmt.Fprintf(os.Stderr, "Error running `go get %s@%s`: %v\n", orchestrionModule, version.Tag, err)
			os.Exit(1)
		}

		fmt.Printf("Successfully added %s@%s to go.mod!", orchestrionModule, version.Tag)

		return
	case "go":
		if err := goproxy.Run(args, goproxy.WithToolexec(orchestrionBinPath, "toolexec")); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v", err)
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

					// We try to open the executable. If that fails, we won't be able to hash it, but we'll
					// ignore this error. The consequence is that GOCACHE entries may be re-used when they
					// shouldn't; which is only a problem on dev iteration. On Windows specifically, this may
					// always fail due to being unable to open a running executable for reading.
					if file, err := os.Open(orchestrionBinPath); err == nil {
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
		"pin",
	}
	fmt.Printf("Usage:\n    %s <command> [arguments]\n\n", cmd)
	fmt.Println("Available commands:")
	for _, cmd := range commands {
		fmt.Printf("    %s\n", cmd)
	}
}

var (
	orchestrionBinPath string // The path to the current executable file
	requiredVersionOK  bool   // Whether the go.mod version check succeeded
)

func init() {
	// Ensure we're using the correct version of the tooling...
	if err := ensure.RequiredVersion(); err != nil {
		log.Debugf("Failed to detect required version fo orchestrion from go.mod: %v\n", err)
	} else {
		requiredVersionOK = true
	}

	var err error
	if orchestrionBinPath, err = os.Executable(); err != nil {
		if orchestrionBinPath, err = filepath.Abs(os.Args[0]); err != nil {
			orchestrionBinPath = os.Args[0]
		}
	}
	orchestrionBinPath = path.Clean(orchestrionBinPath)
}
