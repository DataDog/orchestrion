// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package utils

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path"
	"strings"
	"time"
)

// ExitIfError calls os.Exit(1) if err is not nil
func ExitIfError(err error) {
	if err == nil {
		return
	}
	if err, ok := err.(*exec.ExitError); ok {
		log.Println(err)
		// We'll use the same exit code...
		os.Exit(err.ExitCode())
	}
	log.Fatalln("unexpected error:", err)
}

// GoBuild configures an execution of the `go build` command managed by this
// process. Builds always run with the `-work` and `-a` flags.
type GoBuild struct {
	Dir            string   // The directory in context of which the build runs
	ImportPath     string   // The import path of the package being built
	ExtraArgs      []string // Additional arguments to provide to the build
	TempDir        string   // The temporary directory to use for the build
	ExtraEnv       []string // Additional environment to pass to the child process
	Stdout, Stderr io.Writer
}

// Run executes the configured `go build` command, and if successful returns the
// build's WORK directory path.
func (g GoBuild) Run() (string, error) {
	if g.Stdout == nil {
		g.Stdout = os.Stdout
	}
	if g.Stderr == nil {
		g.Stderr = os.Stderr
	}

	cmd := exec.Command("go", "build", "-work")
	cmd.Args = append(cmd.Args, "-toolexec", fmt.Sprintf("%s toolexec", orchestrionBin))
	cmd.Args = append(cmd.Args, g.ExtraArgs...)
	cmd.Args = append(cmd.Args, g.ImportPath)
	cmd.Dir = g.Dir

	stderr := bytes.NewBuffer(nil)
	cmd.Stderr = io.MultiWriter(g.Stderr, stderr)
	cmd.Stdout = g.Stdout

	cmd.Env = append(os.Environ(), g.ExtraEnv...)
	if g.TempDir != "" {
		cmd.Env = append(cmd.Env,
			fmt.Sprintf("TMPDIR=%s", g.TempDir),
			// To cater for alternate environment variable names...
			fmt.Sprintf("TEMP=%s", g.TempDir),
			fmt.Sprintf("TEMPDIR=%s", g.TempDir),
			fmt.Sprintf("TMP=%s", g.TempDir),
		)
	}

	log.Printf("Starting child process: %q\n", cmd.Args)
	printed := make(map[string]struct{})
	for i := len(cmd.Env) - 1; i >= 0; i-- {
		name, val, _ := strings.Cut(cmd.Env[i], "=")
		if _, found := printed[name]; !found && (name == "TMPDIR" || strings.Contains(name, "ORCHESTRION")) {
			log.Printf("... %s=%s\n", name, val)
			printed[name] = struct{}{}
		}
	}

	start := time.Now()
	if err := cmd.Run(); err != nil {
		dur := time.Since(start)
		log.Printf("Failed after %s\n", dur)
		if err, ok := err.(*exec.ExitError); ok {
			err.Stderr = stderr.Bytes()
		}
		return "", fmt.Errorf("child build failed: %w", err)
	}
	dur := time.Since(start)
	log.Printf("Completed in %s\n", dur)

	out := stderr.String()
	work, wDir, ok := strings.Cut(out, "=")
	if !ok || work != "WORK" {
		return "", fmt.Errorf("unexpected output of go build -work: %q", out)
	}

	wDir, _, _ = strings.Cut(wDir, "\n")

	return strings.TrimSpace(wDir), nil
}

var orchestrionBin string

func init() {
	exe, err := os.Executable()
	if err != nil {
		exe = os.Args[0]
	}
	orchestrionBin = path.Clean(exe)
}
