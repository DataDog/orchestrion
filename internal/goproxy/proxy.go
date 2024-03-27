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
	"runtime"
	"strings"
	"syscall"

	"github.com/datadog/orchestrion/internal/injector/builtin"
	"github.com/datadog/orchestrion/internal/version"
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
	return func(c *config) {
		c.toolexecArgs = strings.Join(args, " ")
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
	case "build", "run", "test":
		if cfg.forceBuild {
			args = append([]string{cmd, "-a"}, args[1:]...)
		}
		if cfg.toolexecArgs != "" {
			args = append([]string{cmd, "-toolexec", cfg.toolexecArgs}, args[1:]...)
		}
		fallthrough
	case "env":
		if cfg.differentCache {
			goCache, err := Goenv(goCacheVar)
			if err != nil {
				return err
			}
			cacheVar := fmt.Sprintf("%s=%s.orchestrion-%s@%s", goCacheVar, goCache, version.Tag, builtin.Checksum[:8])
			env = append(env, cacheVar)
		}
	default:
		break
	}

	args = append([]string{goBin}, args...)

	log.Printf("Executing %q", args)

	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	return syscall.Exec(goBin, args, env)
}
