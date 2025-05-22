// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2025 Datadog, Inc.

package report

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"

	"github.com/DataDog/orchestrion/internal/injector/parse"
	"github.com/DataDog/orchestrion/internal/toolexec/aspect"
	"github.com/rs/zerolog"
	"github.com/sergi/go-diff/diffmatchpatch"
	"golang.org/x/sync/errgroup"
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
func (r Report) Diff(ctx context.Context, writer io.Writer) error {
	dmp := diffmatchpatch.New()
	log := zerolog.Ctx(ctx)

	var (
		wg      errgroup.Group
		diffs   []diffmatchpatch.Diff
		diffsMu sync.Mutex
	)

	for _, modifiedPath := range r.Files {
		wg.Go(func() error {
			modifiedFile, err := os.Open(modifiedPath)
			if err != nil {
				return fmt.Errorf("read %s: %w", modifiedPath, err)
			}

			defer modifiedFile.Close()

			originalPath, err := parse.ConsumeLineDirective(modifiedFile)
			if err != nil {
				return fmt.Errorf("consume line directive: %w", err)
			}

			modifiedCode, err := io.ReadAll(modifiedFile)
			if err != nil {
				return fmt.Errorf("read %s: %w", modifiedPath, err)
			}

			var originalCode []byte
			if originalPath != "" {
				originalCode, err = os.ReadFile(originalPath)
				if err != nil {
					return fmt.Errorf("read %s: %w", originalPath, err)
				}
			}

			// TODO: work with charmaps to avoid converting to string and support multiple encodings
			originalRunes, modifiedRunes, _ := dmp.DiffLinesToRunes(string(originalCode), string(modifiedCode))
			fragments := dmp.DiffMainRunes(originalRunes, modifiedRunes, false)
			fragments = dmp.DiffCleanupEfficiency(fragments)
			fragments = dmp.DiffCleanupSemantic(fragments)
			diffsMu.Lock()
			defer diffsMu.Unlock()
			diffs = append(diffs, fragments...)
			return nil
		})
	}

	if err := wg.Wait(); err != nil {
		log.Error().Err(err).Msg("failed to generate diff")
	}

	output := dmp.DiffPrettyText(diffs)
	length := len(output)

	for {
		if length == 0 {
			break
		}
		n, err := io.WriteString(writer, output)
		if err != nil {

			return err
		}
		length -= n
	}

	return nil
}
