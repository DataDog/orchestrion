// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package processors

import (
	"errors"
	"fmt"
	"os"
	"path"
	"strings"
)

// IndexCompileUnit creates a new semantic symbolic link for the package being
// compiled in the work directory, pointing to the command's stage directory. It
// returns a boolean indicating whether the symbolic link was created by this
// process, meaning it owns building this package; or another, meaning it should
// wait for the other process to be done with its business.
func IndexCompileUnit(importPath, stageDir string) (linkname string, owned bool, err error) {
	if importPath == "" {
		return "", false, nil
	}
	if importPath == "main" {
		// We don't index the main package -- there could be several of them!
		return "", true, nil
	}

	root := os.Getenv(envVarOrchestrionRootBuild)
	if root == "" {
		root = path.Dir(stageDir)
	}

	linkname = path.Join(root, slugify(importPath))
	err = os.Symlink(stageDir, linkname)
	if err != nil {
		if errors.Is(err, os.ErrExist) {
			// Another process created that symlink, we did not.
			owned, err := isSameDir(linkname, stageDir)
			return linkname, owned, err
		}
		return linkname, false, err
	}
	// This process created the symlink, and "owns" this build.
	return linkname, true, nil
}

func LookupCompileUnit(importPath string, stageDir string) string {
	root := os.Getenv(envVarOrchestrionRootBuild)
	if root == "" {
		root = path.Dir(stageDir)
	}

	linkname := path.Join(root, slugify(importPath))
	if _, err := os.Stat(linkname); err != nil {
		return "" // Not found
	}

	return linkname
}

func slugify(s string) string {
	return "-" + strings.ReplaceAll(s, "/", "-")
}

func isSameDir(left, right string) (bool, error) {
	leftStat, err := os.Stat(left)
	if err != nil {
		return false, fmt.Errorf("failed stat(%q): %w", left, err)
	}

	rightStat, err := os.Stat(right)
	if err != nil {
		return false, fmt.Errorf("failed stat(%q): %w", right, err)
	}

	return os.SameFile(leftStat, rightStat), nil
}
