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
		switch proxyCmd.Type() {
		case proxy.CommandTypeCompile, proxy.CommandTypeLink:
			// We only do `autoPinOrchestrion` in case the command is a "known" and interesting one. It is
			// otherwise a waste of time and risks failing due to working directory issues. For example,
			// the `asm` commands run with the current working directory set to the source tree of the
			// package, whereas `compile` and `link` run in the directrory where the `go.mod` file is.
			autoPinOrchestrion()
		}

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
	case "warmup":
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
			fmt.Fprintf(os.Stderr, "Unable to initialize temporary module: %v\n", err)
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
			fmt.Fprintf(os.Stderr, "Unable to install orchestrion in temporary module: %v\n", err)
			if stderr.Len() > 0 {
				fmt.Fprintf(os.Stderr, "Error output:\n%s\n", stderr.String())
			}
			os.Exit(1)
		}

		log.Tracef("Running `go build -v <modules>` in the temporary module...\n")
		buildArgs := make([]string, 0, 2+len(args)+11)
		buildArgs = append(buildArgs, "go", "build")
		buildArgs = append(buildArgs, args...)
		buildArgs = append(buildArgs,
			// All packages we may be instrumenting, plus the standard library.
			"github.com/datadog/orchestrion/instrument",
			"gopkg.in/DataDog/dd-trace-go.v1/contrib/database/sql",
			"gopkg.in/DataDog/dd-trace-go.v1/contrib/gin-gonic/gin",
			"gopkg.in/DataDog/dd-trace-go.v1/contrib/go-chi/chi.v5",
			"gopkg.in/DataDog/dd-trace-go.v1/contrib/gofiber/fiber.v2",
			"gopkg.in/DataDog/dd-trace-go.v1/contrib/google.golang.org/grpc",
			"gopkg.in/DataDog/dd-trace-go.v1/contrib/gorilla/mux",
			"gopkg.in/DataDog/dd-trace-go.v1/contrib/labstack/echo.v4",
			"gopkg.in/DataDog/dd-trace-go.v1/ddtrace",
			"gopkg.in/DataDog/dd-trace-go.v1/ddtrace/tracer",
			"std",
		)
		cmd = exec.Command(orchestrionBinPath, buildArgs...)
		cmd.Dir = tmp
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			fmt.Fprintf(os.Stderr, "Warm-up build failed: %v\n", err)
			os.Exit(1)
		}
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
	orchestrionBinPath = path.Clean(orchestrionBinPath)
}
