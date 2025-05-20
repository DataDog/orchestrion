// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package cmd

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/DataDog/orchestrion/internal/toolexec/aspect"
	"github.com/rs/zerolog"
	"github.com/sergi/go-diff/diffmatchpatch"
	"github.com/urfave/cli/v2"
	"golang.org/x/sync/errgroup"
)

var Diff = &cli.Command{
	Name:  "diff",
	Usage: "Generates a diff between a nominal and orchestrion-instrumented build using a go work directory that can be obtained running `orchestrion go build -work -a`. This does work with -cover builds.",
	Args:  true,
	Action: func(clictx *cli.Context) (err error) {
		workFolder := clictx.Args().First()
		if workFolder == "" {
			return cli.ShowSubcommandHelp(clictx)
		}

		report, err := reportFromWorkDir(clictx.Context, workFolder)
		if err != nil {
			return cli.Exit(fmt.Sprintf("failed to read work dir: %s (did you forgot the -work flag during build ?)", err), 1)
		}

		if len(report.Files) == 0 {
			return cli.Exit("no files to diff (did you forgot the -a flag during build?)", 1)
		}

		if err := report.diff(clictx.App.Writer); err != nil {
			return cli.Exit(fmt.Sprintf("failed to generate diff: %s", err), 1)
		}

		return nil
	},
}

func reportFromWorkDir(ctx context.Context, dir string) (report, error) {
	log := zerolog.Ctx(ctx).With().Str("work-dir", dir).Logger()

	entries, err := os.ReadDir(dir)
	if err != nil {
		return report{}, fmt.Errorf("read dir %s: %w", dir, err)
	}

	rp := report{
		Files: make(map[string]string),
	}

	for _, packageBuildDir := range entries {
		if !packageBuildDir.IsDir() || !strings.HasPrefix(packageBuildDir.Name(), "b") {
			log.Debug().Str("package-dir", packageBuildDir.Name()).Msg("skipping build dir entry")
			continue
		}

		orchestrionDir := filepath.Join(dir, packageBuildDir.Name(), aspect.OrchestrionDirPathElement)
		if err := filepath.WalkDir(orchestrionDir, func(path string, d os.DirEntry, err error) error {
			if err != nil {
				return fmt.Errorf("walk dir %s: %w", path, err)
			}

			if d.IsDir() || !strings.HasPrefix(d.Name(), ".go") {
				return nil
			}

			originalPath, ok := aspect.OriginalFilePath(path)
			if !ok {
				log.Debug().Str("path", path).Msg("skipping unknown file")
				return nil
			}

			rp.Files[originalPath] = path
			log.Debug().Str("original-path", originalPath).Str("orchestrion-path", path).Msg("found orchestrion file")
			return nil
		}); err != nil {
			log.Debug().Err(err).Str("orchestrion-dir", orchestrionDir).Msg("failed to walk orchestrion dir")
		}
	}

	return rp, nil
}

type report struct {
	Files map[string]string
}

func (r report) diff(writer io.Writer) error {
	dmp := diffmatchpatch.New()

	var (
		wg      errgroup.Group
		diffs   []diffmatchpatch.Diff
		diffsMu sync.Mutex
	)

	for originalPath, modifiedPath := range r.Files {
		wg.Go(func() error {
			source, err := os.ReadFile(originalPath)
			if err != nil {
				return fmt.Errorf("read %s: %w", originalPath, err)
			}

			target, err := os.ReadFile(modifiedPath)
			if err != nil {
				return fmt.Errorf("read %s: %w", modifiedPath, err)
			}

			// TODO: work with charmaps to avoid converting to string and support multiple encodings
			fragments := dmp.DiffMainRunes([]rune(string(source)), []rune(string(target)), false)
			fragments = dmp.DiffCleanupEfficiency(fragments)
			fragments = dmp.DiffCleanupSemantic(fragments)
			diffsMu.Lock()
			defer diffsMu.Unlock()
			diffs = append(diffs, fragments...)
			return nil
		})
	}

	if err := wg.Wait(); err != nil {
		return err
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
