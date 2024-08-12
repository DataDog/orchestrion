// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package ensure

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"syscall"

	"github.com/datadog/orchestrion/internal/log"
	"github.com/datadog/orchestrion/internal/version"
	"golang.org/x/tools/go/packages"
)

const (
	orchestrionPkgPath    = "github.com/datadog/orchestrion"
	envVarRespawnedFor    = "DD_ORCHESTRION_RESPAWNED_FOR"
	envVarStartupVersion  = "DD_ORCHESTRION_STARTUP_VERSION"
	envValRespawnReplaced = "<replaced>"
)

var (
	errRespawnLoop    = errors.New("re-spawn loop detected")
	orchestrionSrcDir string
)

// RequiredVersion makes sure the version of the tool currently running is the same as the one
// required in the current working directory's "go.mod" file by calling `syscall.Exec` with the
// relevant `go run` command if necessary to replace the current process with one using the required
// version.
//
// If this returns `nil`, the current process is running the correct version of the tool and can
// proceed with it's intended purpose. If it returns an error, that should be presented to the user
// before exiting with a non-0 status code. If the process was correctly substituted, this function
// never returns control to its caller (as the process has been replaced).
func RequiredVersion() error {
	return requiredVersion(goModVersion, os.Getenv, syscall.Exec, os.Args)
}

// StartupVersion returns the version of Orchestrion that has started this process. If this is the
// same as version.Tag, this process hasn't needed to be re-started. This is useful to provide
// complete information about proxied executions (e.g: in the output of `orchestrion version`),
// in cases where a "globally" installed binary substituted itself for a version from `go.mod`.
func StartupVersion() string {
	if env := os.Getenv(envVarStartupVersion); env != "" {
		return env
	}
	return version.Tag
}

// requiredVersion is the internal implementation of RequiredVersion, and takes the goModVersion and
// syscall.Exec functions as arguments to allow for easier testing. Panics if `osArgs` is 0-length.
func requiredVersion(
	goModVersion func(string) (string, string, error),
	osGetenv func(string) string,
	syscallExec func(argv0 string, argv []string, env []string) error,
	osArgs []string,
) error {
	rVersion, path, err := goModVersion("" /* Current working directory */)
	if err != nil {
		return fmt.Errorf("failed to determine go.mod requirement for %q: %w", orchestrionPkgPath, err)
	}

	if rVersion == version.Tag || (rVersion == "" && path == orchestrionSrcDir) {
		// This is the correct version already, so we can proceed without further ado.
		return nil
	}

	if respawn := osGetenv(envVarRespawnedFor); respawn != "" && respawn != envValRespawnReplaced {
		// We're already re-spawning for a non-local version, so we should not be re-spawning again...
		// If that were the case, we'd likely end up in an infinite loop of re-spawning, which is very
		// much undesirable.
		return fmt.Errorf(
			"%w (wanted %s, got %s, already respawning for %s)",
			errRespawnLoop,
			rVersion,
			version.Tag,
			respawn,
		)
	}

	if rVersion == "" {
		// If there is no required version, it means a replace directive is in use, and it does not
		// macth the running process' original source tree, so we will unconditionally re-spawn.
		rVersion = envValRespawnReplaced
	}

	log.Infof("Re-starting with '%s@%s' (this is %s)\n", orchestrionPkgPath, rVersion, version.Tag)

	goBin, err := exec.LookPath("go")
	if err != nil {
		return fmt.Errorf("failed to resolve go from PATH: %w", err)
	}

	if len(osArgs) == 0 {
		panic("received 0-length osArgs, which is not supposed to happen")
	}

	args := make([]string, len(osArgs)+2)
	args[0] = goBin
	args[1] = "run"
	args[2] = orchestrionPkgPath
	copy(args[3:], osArgs[1:])

	env := os.Environ()
	env = append(
		env,
		fmt.Sprintf("%s=%s", envVarRespawnedFor, rVersion),
		fmt.Sprintf("%s=%s", envVarStartupVersion, version.Tag),
	)

	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	// Won't return control if successful, never returns a `nil` error value.
	return syscallExec(goBin, args, env)
}

// goModVersion returns the version and path of the "github.com/datadog/orchestrion" module that is
// required in the specified directory's "go.mod" file. If dir is blank, the process' current
// working directory is used. The version may be blank if a replace directive is in effect; in which
// case the path value may indicate the location of the source code that is being used instead.
func goModVersion(dir string) (moduleVersion string, moduleDir string, err error) {
	pkgs, err := packages.Load(
		&packages.Config{
			Dir:  dir,
			Mode: packages.NeedModule,
			Logf: func(format string, args ...any) { log.Tracef(format+"\n", args...) },
		},
		orchestrionPkgPath,
	)
	if err != nil {
		return "", "", err
	}

	pkg := pkgs[0]
	if len(pkg.Errors) > 0 {
		errs := make([]error, len(pkg.Errors))
		for i, e := range pkg.Errors {
			errs[i] = errors.New(e.Error())
		}
		return "", "", errors.Join(errs...)
	}

	if pkg.Module.Replace != nil {
		// If there's a replace directive, that's what we need to be honoring instead.
		return pkg.Module.Replace.Version, pkg.Module.Replace.Dir, nil
	}

	return pkg.Module.Version, pkg.Module.Dir, nil
}

func init() {
	_, file, _, _ := runtime.Caller(0)
	orchestrionSrcDir = filepath.Dir(filepath.Dir(filepath.Dir(file)))
}
