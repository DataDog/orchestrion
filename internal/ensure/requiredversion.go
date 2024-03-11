// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package ensure

import (
	"errors"
	"fmt"
	"log"
	"os"
	"syscall"

	"github.com/datadog/orchestrion/internal/version"
	"golang.org/x/tools/go/packages"
)

const (
	orchestrionPkgPath = "github.com/datadog/orchestrion"
	envVarRespawned    = "DD_ORCHESTRION_RESPAWNED_FOR"
	envValRespawnLocal = "<local>"
)

// EnsureRequiredVersion makes sure the version of the tool currently running is the same as the one
// required in the current working directory's "go.mod" file by calling `syscall.Exec` with the
// relevant `go run` command if necessary to replace the current process with one using the required
// version.
//
// If this returns `nil`, the current process is running the correct version of the tool and can
// proceed with it's intended purpose. If it returns an error, that should be presented to the user
// before exiting with a non-0 status code. If the process was correctly substituted, this function
// never returns control to its caller (as the process has been replaced).
func RequiredVersion() error {
	required, err := goModVersion()
	if err != nil {
		return fmt.Errorf("failed to determine go.mod requirement for %q: %w", orchestrionPkgPath, err)
	}

	if required == version.Tag {
		// This is the correct version or no specific version could be determined (indicating a dev/replaced package is in
		// use), so we can proceed without further ado.
		return nil
	}

	if respawn := os.Getenv(envVarRespawned); respawn != "" && respawn != envValRespawnLocal {
		// We're already re-spawning for a non-local version, so we should not be re-spawning again...
		// If that were the case, we'd likely end up in an infinite loop of re-spawning, which is very
		// much undesirable.
		return fmt.Errorf(
			"re-spawn loop detected (wanted %s, got %s, already respawning for %s)",
			required,
			version.Tag,
			respawn,
		)
	}

	if required == "" {
		// If there is no required version, it means a local version is used instead, either because we
		// are in Orchestrion's own development tree, or because the user has introduced a "replace"
		// directive for orchestion. In such cases, we unconditionally exec `go run` exactly once.
		required = envValRespawnLocal
	}

	log.Printf("Re-starting with '%s@%s' (this is %s)\n", orchestrionPkgPath, required, version.Tag)

	args := make([]string, len(os.Args)+1)
	args[0] = "run"
	args[1] = orchestrionPkgPath
	copy(args[2:], os.Args[1:])

	env := os.Environ()
	env = append(env, fmt.Sprintf("%s=%s", envVarRespawned, required))

	err = syscall.Exec("go", args, env)
	return fmt.Errorf("failed to exec `go run %s ...`: %w", orchestrionPkgPath, err)
}

// goModVersion returns the version of the "github.com/datadog/orchestrion" module that is required
// in the current working directory's "go.mod" file. The versions may be blank, indicating a replace
// directive redirects the package to a local source tree.
func goModVersion() (string, error) {
	cfg := &packages.Config{Mode: packages.NeedModule}
	pkgs, err := packages.Load(cfg, orchestrionPkgPath)
	if err != nil {
		return "", err
	}

	pkg := pkgs[0]
	if len(pkg.Errors) > 0 {
		errs := make([]error, len(pkg.Errors))
		for i, e := range pkg.Errors {
			errs[i] = errors.New(e.Error())
		}
		return "", errors.Join(errs...)
	}

	return pkg.Module.Version, nil
}
