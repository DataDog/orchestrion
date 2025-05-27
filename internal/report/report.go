// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2025 Datadog, Inc.

package report

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"iter"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/DataDog/orchestrion/internal/injector/parse"
	"github.com/DataDog/orchestrion/internal/toolexec/aspect"
	"github.com/rs/zerolog"
)

// FromWorkDir reads the orchestrion files from a `go build -work` directory and creates a [Report] out of it.
func FromWorkDir(ctx context.Context, dir string) (Report, error) {
	log := zerolog.Ctx(ctx).With().Str("work-dir", dir).Logger()

	entries, err := os.ReadDir(dir)
	if err != nil {
		return Report{}, fmt.Errorf("read dir %s: %w", dir, err)
	}

	rp := Report{}
	for _, packageBuildDir := range entries {
		orchestrionDir := filepath.Join(dir, packageBuildDir.Name(), aspect.OrchestrionDirPathElement)
		_ = filepath.WalkDir(orchestrionDir, func(path string, d os.DirEntry, err error) error {
			if err != nil {
				return fmt.Errorf("walk dir %s: %w", path, err)
			}

			if d.IsDir() || !strings.HasSuffix(d.Name(), ".go") {
				return nil
			}

			log.Debug().Str("path", path).Msg("found orchestrion file")
			rp.Files = append(rp.Files, path)
			return nil
		})
	}

	sort.Strings(rp.Files)

	return rp, nil
}

type Report struct {
	Files []string
}

// WithFilter filters the files in the report based on a regex pattern.
func (r Report) WithFilter(regex string) (Report, error) {
	cmpRegex, err := regexp.Compile(regex)
	if err != nil {
		return Report{}, fmt.Errorf("invalid regex %q: %w", regex, err)
	}

	var filteredFiles []string
	for _, file := range r.Files {
		if cmpRegex.MatchString(file) {
			filteredFiles = append(filteredFiles, file)
		}
	}
	return Report{Files: filteredFiles}, nil
}

// Diff generates a diff between the original and modified files and writes it to the writer.
func (r Report) Diff(writer io.Writer) error {
	var errs []error

	// Check if the diff command is available, exit early if not
	if _, err := exec.LookPath("diff"); err != nil {
		return fmt.Errorf("diff command not found: %w (cannot run orchestrion diff without the diff binary being in the path)", err)
	}

	for _, modifiedPath := range r.Files {
		if err := r.diff(writer, modifiedPath); err != nil {
			errs = append(errs, err)
		}
	}

	return errors.Join(errs...)
}

func (r Report) diff(writer io.Writer, modifiedPath string) error {
	modifiedFile, err := os.Open(modifiedPath)
	if err != nil {
		return fmt.Errorf("open %s: %w", modifiedPath, err)
	}

	defer modifiedFile.Close()

	originalPath, err := parse.ConsumeLineDirective(modifiedFile)
	if err != nil {
		return fmt.Errorf("consume line directive: %w", err)
	}

	// If originalPath does not exists, it means that we have cgo files in there, just skip it
	if _, err := os.Stat(originalPath); os.IsNotExist(err) {
		return nil
	}

	cmd := exec.Command("diff",
		"-u",            // Unified diff format
		"-d",            // Try harder to minimize the diff
		"-a",            // Treat all files as text
		"--color",       // Change the output to colored diff if possible
		"-w",            // Ignore whitespace changes
		"-E",            // Ignore tab characters
		"-B",            // Ignore blank lines
		"-I", "^//line", // Don't print line directives in the diff when they would end up being alone in a fragment
		originalPath, modifiedPath)

	var buf bytes.Buffer

	cmd.Stdout = writer
	cmd.Stderr = &buf

	if err := cmd.Run(); err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			if exitErr.ExitCode() == 1 {
				return nil // Differences were found, thanks sherlock.
			}
		}
		return fmt.Errorf("running diff command: %w (stderr: %s)", err, buf.String())
	}

	return nil
}

// Packages returns an iterator over the unique package names found in the report.
// It extracts the package names from the file paths, assuming they follow the convention of being
// constructed as "<work-dir>/orchestrion/src/<github.com/my/repo>/<file.go>".
func (r Report) Packages() iter.Seq[string] {
	return func(yield func(string) bool) {
		pkgs := make(map[string]bool)
		for _, file := range r.Files {
			dir := filepath.Dir(file)
			_, pkg, found := strings.Cut(dir, "orchestrion/src/")
			if !found {
				continue
			}

			pkg = strings.Trim(pkg, "/")
			if pkgs[pkg] {
				continue
			}

			pkgs[pkg] = true
			if !yield(pkg) {
				return
			}
		}
	}
}
