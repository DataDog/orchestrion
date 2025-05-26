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
	"slices"
	"strings"
	"sync"

	"github.com/DataDog/orchestrion/internal/injector/parse"
	"github.com/DataDog/orchestrion/internal/toolexec/aspect"
	"github.com/rs/zerolog"
	"github.com/sergi/go-diff/diffmatchpatch"
	"github.com/sourcegraph/go-diff/diff"
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
	log := zerolog.Ctx(ctx)

	var (
		wg    errgroup.Group
		diffs []*diff.FileDiff
		mu    sync.Mutex
	)

	for _, modifiedPath := range r.Files {
		wg.Go(func() error {
			hunks, err := diffFile(modifiedPath)
			if err != nil {
				return fmt.Errorf("generate diff for %s: %w", modifiedPath, err)
			}

			log.Debug().Str("file", modifiedPath).Msg("generated diff for file")

			mu.Lock()
			defer mu.Unlock()
			diffs = append(diffs, hunks)
			return nil
		})
	}

	if err := wg.Wait(); err != nil {
		log.Error().Err(err).Msg("failed to generate diff for some files")
	}

	slices.SortFunc(diffs, func(a, b *diff.FileDiff) int {
		return strings.Compare(a.NewName, b.NewName)
	})

	output, err := diff.PrintMultiFileDiff(diffs)
	if err != nil {
		return fmt.Errorf("print multi file diff: %w", err)
	}

	length := len(output)
	for length != 0 {
		n, err := writer.Write(output)
		if err != nil {
			return err
		}

		length -= n
	}

	return nil
}

// diffFile generates a [diff.FileDiff] for the given orchestrion generated file path
// It uses github.com/sergi/go-diff to compute the diff between the original file and the modified file.
// And then uses github.com/sourcegraph/go-diff to read the hunks from the diff output.
// Which allows us to create a context aware diff that can be used to print the diff in a human readable way.
func diffFile(modifiedPath string) (*diff.FileDiff, error) {
	dmp := diffmatchpatch.New()
	modifiedFile, err := os.Open(modifiedPath)
	if err != nil {
		return nil, fmt.Errorf("open %s: %w", modifiedPath, err)
	}

	defer modifiedFile.Close()

	originalPath, err := parse.ConsumeLineDirective(modifiedFile)
	if err != nil {
		return nil, fmt.Errorf("consume line directive: %w", err)
	}

	modifiedCode, err := io.ReadAll(modifiedFile)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", modifiedPath, err)
	}

	var originalCode []byte
	if originalPath != "" {
		originalCode, err = os.ReadFile(originalPath)
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", originalPath, err)
		}
	}

	// TODO: work with charmaps to avoid converting to string and support multiple encodings
	originalRunes, modifiedRunes, _ := dmp.DiffLinesToRunes(string(originalCode), string(modifiedCode))
	fragments := dmp.DiffMainRunes(originalRunes, modifiedRunes, true)
	fragments = dmp.DiffCleanupEfficiency(fragments)
	fragments = dmp.DiffCleanupSemantic(fragments)

	hunks, err := diffpatchmatchToHunk(fragments)
	if err != nil {
		return nil, fmt.Errorf("read hunks from diff: %w", err)
	}

	return &diff.FileDiff{
		NewName:  modifiedPath,
		OrigName: originalPath,
		Hunks:    hunks,
	}, nil
}

func diffpatchmatchToHunk(fragments []diffmatchpatch.Diff) ([]*diff.Hunk, error) {
	origLine := 1
	modifiedLine := 1

	var hunks []*diff.Hunk

	for _, fragment := range fragments {
		switch fragment.Type {
		case diffmatchpatch.DiffEqual:
			// No change, just move the lines forward
			origLine += strings.Count(fragment.Text, "\n")
			modifiedLine += strings.Count(fragment.Text, "\n")
		case diffmatchpatch.DiffInsert:
			// Lines added in modified file
			newLines := strings.Split(fragment.Text, "\n")
			newLinesPlus := make([]string, len(newLines))
			for i, line := range newLines {
				newLinesPlus[i] = "+" + line
			}
			hunks = append(hunks, &diff.Hunk{
				OrigStartLine: int32(origLine),
				OrigLines:     0,
				NewStartLine:  int32(modifiedLine),
				NewLines:      int32(len(newLines)),
				Body:          []byte(strings.Join(newLinesPlus, "\n")),
			})
			modifiedLine += len(newLines)
		case diffmatchpatch.DiffDelete:
			// Lines removed from original file
			rmLines := strings.Split(fragment.Text, "\n")
			rmLinesMinus := make([]string, len(rmLines))
			for i, line := range rmLines {
				rmLinesMinus[i] = "-" + line
			}
			hunks = append(hunks, &diff.Hunk{
				OrigStartLine: int32(origLine),
				OrigLines:     int32(len(rmLines)),
				NewStartLine:  int32(modifiedLine),
				NewLines:      0,
				Body:          []byte(strings.Join(rmLinesMinus, "\n")),
			})
			modifiedLine += len(rmLines)
		default:
			return nil, fmt.Errorf("unknown diff type %d", fragment.Type)
		}
	}

	return hunks, nil
}
