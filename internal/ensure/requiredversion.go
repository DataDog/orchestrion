// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package ensure

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"runtime"

	"github.com/DataDog/orchestrion/internal/goenv"
	"github.com/DataDog/orchestrion/internal/version"
	"github.com/rs/zerolog"
	"golang.org/x/tools/go/packages"
)

const orchestrionPkgPath = "github.com/DataDog/orchestrion"

var orchestrionSrcDir string

// IncorrectVersionError is returned by [RequiredVersion] when the version of orchestrion running
// does not match the one required by `go.mod`.
type IncorrectVersionError struct {
	// RequiredVersion is the version declared in `go.mod`, or a blank string if a `replace` directive
	// for "github.com/DataDog/orchestrion" is present in `go.mod`.
	RequiredVersion string
}

// RequiredVersion makes sure the version of the tool currently running is the same as the one
// required in the current working directory's "go.mod" file.
//
// If this returns `nil`, the current process is running the correct version of the tool and can
// proceed with it's intended purpose. If it returns an [IncorrectVersionError], the caller should
// determine whether to print a warning or exit in error, presenting the returned error to the user.
func RequiredVersion(ctx context.Context) error {
	return requiredVersion(ctx, goModVersion)
}

func (e IncorrectVersionError) Error() string {
	const useMessage = "use `go run github.com/DataDog/orchestrion` instead of just `orchestrion`"
	if e.RequiredVersion == "" {
		return "orchestrion is diverted by a replace directive; please " + useMessage
	}
	return fmt.Sprintf(
		"orchestrion@%s is required by `go.mod`, but this is orchestrion@%s - please run `go install github.com/DataDog/orchestrion@%[1]s` or "+useMessage,
		e.RequiredVersion,
		version.Tag(),
	)
}

// requiredVersion is the internal implementation of RequiredVersion, and takes the goModVersion and
// syscall.Exec functions as arguments to allow for easier testing. Panics if `osArgs` is 0-length.
func requiredVersion(
	ctx context.Context,
	goModVersion func(context.Context, string) (string, string, error),
) error {
	rVersion, path, err := goModVersion(ctx, "" /* Current working directory */)
	if err != nil {
		return fmt.Errorf("failed to determine go.mod requirement for %q: %w", orchestrionPkgPath, err)
	}

	rawTag, _ := version.TagInfo()
	if rVersion == rawTag || rVersion == version.Tag() || (rVersion == "" && path == orchestrionSrcDir) {
		// This is the correct version already, so we can proceed without further ado.
		return nil
	}

	return IncorrectVersionError{RequiredVersion: rVersion}
}

// goModVersion returns the version and path of the "github.com/DataDog/orchestrion" module that is
// required in the specified directory's "go.mod" file. If dir is blank, the process' current
// working directory is used. The version may be blank if a replace directive is in effect; in which
// case the path value may indicate the location of the source code that is being used instead.
func goModVersion(ctx context.Context, dir string) (moduleVersion string, moduleDir string, err error) {
	gomod, err := goenv.GOMOD(dir)
	if err != nil {
		return "", "", err
	}

	log := zerolog.Ctx(ctx)
	cfg := &packages.Config{
		Dir:  filepath.Dir(gomod),
		Mode: packages.NeedModule,
		Logf: func(format string, args ...any) { log.Trace().Str("operation", "packages.Load").Msgf(format, args...) },
	}
	pkgs, err := packages.Load(cfg, orchestrionPkgPath)
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

	// Shouldn't happen but does when the current working directory is not
	// part of a go module's source tree.
	// See: https://github.com/golang/go/issues/65816
	if pkg.Module == nil {
		return "", "", fmt.Errorf("no module information found for package %q", pkg.PkgPath)
	}

	if pkg.Module.Replace != nil {
		// If there's a replace directive, that's what we need to be honoring instead.
		return pkg.Module.Replace.Version, pkg.Module.Replace.Dir, nil
	}

	return pkg.Module.Version, pkg.Module.Dir, nil
}

func init() {
	_, thisFile, _, _ := runtime.Caller(0)
	orchestrionSrcDir = filepath.Join(thisFile, "..", "..", "..")
}
