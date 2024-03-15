// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package goproxy

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"strings"
	"syscall"
)

type config struct {
	differentCache bool
	forceBuild     bool
	toolexecArgs   string
}

type Option func(*config)

const goCacheVar = "GOCACHE"

// WithForceBuild forces a call to Run() to build all go files and dependencies
// when wrapping a `go build` command. It effectively adds the `-a` flag to the build
func WithForceBuild() Option {
	return func(c *config) {
		c.forceBuild = true
	}
}

// WithDifferentCache forces a call to Run() to override the GOCACHE directory
// A temporary directory is instead created and all build artifacts are cached there
func WithDifferentCache() Option {
	return func(c *config) {
		c.differentCache = true
	}
}

// WithToolexec forces a call to Run() to build with the -toolexec option when
// wrapping a build command
func WithToolexec(args []string) Option {
	var sb strings.Builder
	return func(c *config) {
		for _, arg := range args {
			sb.WriteString(fmt.Sprintf("%s ", arg))
		}
		c.toolexecArgs = sb.String()
	}
}

// Run takes a go directive (go build, go install, etc...) and applies
// changes specified through opts to the command before running it in a
// different process
func Run(args []string, opts ...Option) error {
	var cfg config
	env := os.Environ()
	for _, opt := range opts {
		opt(&cfg)
	}
	if len(args) == 0 {
		return fmt.Errorf("no go command provided")
	}
	goBin, err := exec.LookPath("go")
	if err != nil {
		return err
	}

	cmd := args[0]
	switch cmd {
	case "build", "run":
		if cfg.forceBuild {
			args = append([]string{cmd, "-a"}, args[1:]...)
		}
		if len(cfg.toolexecArgs) > 0 {
			args = append([]string{cmd, "-toolexec", cfg.toolexecArgs}, args[1:]...)
		}
		if cfg.differentCache {
			dirPath, err := os.MkdirTemp("", ".goproxy_cache*")
			if err != nil {
				return err
			}
			cacheVar := fmt.Sprintf("%s=%s", goCacheVar, dirPath)
			env = append(env, cacheVar)
		}
	default:
		break
	}

	args = append([]string{goBin}, args...)
	log.Printf("Executing '%v'", args)
	return syscall.Exec(goBin, args, env)
}
