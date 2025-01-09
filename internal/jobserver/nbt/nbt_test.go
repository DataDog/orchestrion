// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package nbt

import (
	"context"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test(t *testing.T) {
	ctx := context.Background()
	if deadline, ok := t.Deadline(); ok {
		var cancel func()
		ctx, cancel = context.WithDeadline(context.Background(), deadline)
		defer cancel()
	}

	const importPath = "github.com/DataDog/orchestrion.test"
	buildID := uuid.NewString()

	t.Run("not-started", func(t *testing.T) {
		subject := &service{dir: t.TempDir()}
		res, err := subject.finish(ctx, FinishRequest{ImportPath: importPath, FinishToken: "bazinga"})
		require.ErrorContains(t, err, "no build started")
		require.Nil(t, res)
	})

	t.Run("start-reuse-finish", func(t *testing.T) {
		subject := &service{dir: t.TempDir()}

		start, err := subject.start(ctx, StartRequest{ImportPath: importPath, BuildID: buildID})
		require.NoError(t, err)
		require.NotEmpty(t, start.FinishToken)
		assert.Empty(t, start.ArchivePath)

		archiveContent := uuid.NewString()
		extraFileContent := uuid.NewString()
		const label Label = "extra.file"

		var wg sync.WaitGroup
		defer wg.Wait()
		for range 10 {
			wg.Add(1)
			go func() {
				defer wg.Done()

				res, err := subject.start(ctx, StartRequest{ImportPath: importPath, BuildID: buildID})
				assert.NoError(t, err)
				assert.Empty(t, res.FinishToken)
				assert.NotEmpty(t, res.ArchivePath)

				content, err := os.ReadFile(res.ArchivePath)
				assert.NoError(t, err)
				assert.Equal(t, archiveContent, string(content))

				assert.Len(t, res.AdditionalFiles, 1)
				path, ok := res.AdditionalFiles[label]
				assert.True(t, ok)
				content, err = os.ReadFile(path)
				assert.NoError(t, err)
				assert.Equal(t, extraFileContent, string(content))
			}()
		}

		archive := filepath.Join(t.TempDir(), "_pkg_.a")
		require.NoError(t, os.WriteFile(archive, []byte(archiveContent), 0o644))

		extraFile := filepath.Join(t.TempDir(), "extra.file")
		require.NoError(t, os.WriteFile(extraFile, []byte(extraFileContent), 0o644))

		res, err := subject.finish(ctx, FinishRequest{
			ImportPath:      importPath,
			FinishToken:     start.FinishToken,
			ArchivePath:     archive,
			AdditionalFiles: map[Label]string{label: extraFile},
		})
		require.NoError(t, err)
		require.NotNil(t, res)
	})

	t.Run("start-conflict-finish", func(t *testing.T) {
		subject := &service{dir: t.TempDir()}

		start, err := subject.start(ctx, StartRequest{ImportPath: importPath, BuildID: buildID})
		require.NoError(t, err)
		require.NotEmpty(t, start.FinishToken)
		assert.Empty(t, start.ArchivePath)

		archiveContent := uuid.NewString()

		var wg sync.WaitGroup
		defer wg.Wait()
		for range 10 {
			wg.Add(1)
			go func() {
				defer wg.Done()

				res, err := subject.start(ctx, StartRequest{ImportPath: importPath, BuildID: buildID + "-alt"})
				assert.ErrorContains(t, err, buildID)
				assert.Nil(t, res)
			}()
		}

		archive := filepath.Join(t.TempDir(), "_pkg_.a")
		require.NoError(t, os.WriteFile(archive, []byte(archiveContent), 0o644))

		res, err := subject.finish(ctx, FinishRequest{
			ImportPath:  importPath,
			FinishToken: start.FinishToken,
			ArchivePath: archive,
		})
		require.NoError(t, err)
		require.NotNil(t, res)
	})

	t.Run("start-finish-finish", func(t *testing.T) {
		const importPath = "github.com/DataDog/orchestrion.test"
		subject := &service{dir: t.TempDir()}

		start, err := subject.start(ctx, StartRequest{ImportPath: importPath, BuildID: buildID})
		require.NoError(t, err)
		require.NotEmpty(t, start.FinishToken)
		assert.Empty(t, start.ArchivePath)

		archiveContent := uuid.NewString()
		archive := filepath.Join(t.TempDir(), "_pkg_.a")
		require.NoError(t, os.WriteFile(archive, []byte(archiveContent), 0o644))

		for range 10 {
			res, err := subject.finish(ctx, FinishRequest{
				ImportPath:  importPath,
				FinishToken: start.FinishToken,
				ArchivePath: archive,
			})
			require.NoError(t, err)
			require.NotNil(t, res)
		}
	})

	t.Run("start-badtoken-finish", func(t *testing.T) {
		const importPath = "github.com/DataDog/orchestrion.test"
		subject := &service{dir: t.TempDir()}

		start, err := subject.start(ctx, StartRequest{ImportPath: importPath, BuildID: buildID})
		require.NoError(t, err)
		require.NotEmpty(t, start.FinishToken)
		assert.Empty(t, start.ArchivePath)

		archiveContent := uuid.NewString()
		archive := filepath.Join(t.TempDir(), "_pkg_.a")
		require.NoError(t, os.WriteFile(archive, []byte(archiveContent), 0o644))

		for range 10 {
			res, err := subject.finish(ctx, FinishRequest{
				ImportPath:  importPath,
				FinishToken: uuid.NewString(),
				ArchivePath: archive,
			})
			require.Error(t, err, "invalid finish token")
			require.Nil(t, res)
		}

		res, err := subject.finish(ctx, FinishRequest{
			ImportPath:  importPath,
			FinishToken: start.FinishToken,
			ArchivePath: archive,
		})
		require.NoError(t, err)
		require.NotNil(t, res)
	})

	t.Run("start-reuse-error", func(t *testing.T) {
		const importPath = "github.com/DataDog/orchestrion.test"
		subject := &service{dir: t.TempDir()}

		start, err := subject.start(ctx, StartRequest{ImportPath: importPath, BuildID: buildID})
		require.NoError(t, err)
		require.NotEmpty(t, start.FinishToken)
		assert.Empty(t, start.ArchivePath)

		errorText := "simulated failure"

		var wg sync.WaitGroup
		defer wg.Wait()
		for range 10 {
			wg.Add(1)
			go func() {
				defer wg.Done()

				res, err := subject.start(ctx, StartRequest{ImportPath: importPath, BuildID: buildID})
				assert.ErrorContains(t, err, errorText)
				assert.Nil(t, res)
			}()
		}

		res, err := subject.finish(ctx, FinishRequest{
			ImportPath:  importPath,
			FinishToken: start.FinishToken,
			Error:       &errorText,
		})
		require.NoError(t, err)
		require.NotNil(t, res)
	})

	t.Run("start-reuse-bad-response", func(t *testing.T) {
		const importPath = "github.com/DataDog/orchestrion.test"
		subject := &service{dir: t.TempDir()}

		start, err := subject.start(ctx, StartRequest{ImportPath: importPath, BuildID: buildID})
		require.NoError(t, err)
		require.NotEmpty(t, start.FinishToken)
		assert.Empty(t, start.ArchivePath)

		var wg sync.WaitGroup
		defer wg.Wait()
		for range 10 {
			wg.Add(1)
			go func() {
				defer wg.Done()

				res, err := subject.start(ctx, StartRequest{ImportPath: importPath, BuildID: buildID})
				assert.ErrorContains(t, err, errNoArchiveNorError.Error())
				assert.Nil(t, res)
			}()
		}

		res, err := subject.finish(ctx, FinishRequest{
			ImportPath:  importPath,
			FinishToken: start.FinishToken,
		})
		require.ErrorIs(t, err, errNoArchiveNorError)
		require.Nil(t, res)
	})

	t.Run("start-reuse-missing.archive.file", func(t *testing.T) {
		const importPath = "github.com/DataDog/orchestrion.test"
		subject := &service{dir: t.TempDir()}

		start, err := subject.start(ctx, StartRequest{ImportPath: importPath, BuildID: buildID})
		require.NoError(t, err)
		require.NotEmpty(t, start.FinishToken)
		assert.Empty(t, start.ArchivePath)

		// Deliberately non-existent!
		archive := filepath.Join(t.TempDir(), "deliberately-missing", "_pkg_.a")

		var wg sync.WaitGroup
		defer wg.Wait()
		for range 10 {
			wg.Add(1)
			go func() {
				defer wg.Done()

				res, err := subject.start(ctx, StartRequest{ImportPath: importPath, BuildID: buildID})
				assert.ErrorContains(t, err, archive)
				assert.Nil(t, res)
			}()
		}

		res, err := subject.finish(ctx, FinishRequest{
			ImportPath:  importPath,
			FinishToken: start.FinishToken,
			ArchivePath: archive,
		})
		require.ErrorContains(t, err, archive)
		require.Nil(t, res)
	})

	t.Run("start-reuse-missing.extra.file", func(t *testing.T) {
		const importPath = "github.com/DataDog/orchestrion.test"
		subject := &service{dir: t.TempDir()}

		start, err := subject.start(ctx, StartRequest{ImportPath: importPath, BuildID: buildID})
		require.NoError(t, err)
		require.NotEmpty(t, start.FinishToken)
		assert.Empty(t, start.ArchivePath)

		label := Label(uuid.NewString())
		// Deliberately non-existent!
		extraFile := filepath.Join(t.TempDir(), "deliberately-missing", "extra.file")

		var wg sync.WaitGroup
		defer wg.Wait()
		for range 10 {
			wg.Add(1)
			go func() {
				defer wg.Done()

				res, err := subject.start(ctx, StartRequest{ImportPath: importPath, BuildID: buildID})
				assert.ErrorContains(t, err, extraFile)
				assert.Nil(t, res)
			}()
		}

		archive := filepath.Join(t.TempDir(), "_pkg_.a")
		require.NoError(t, os.WriteFile(archive, []byte(uuid.NewString()), 0o644))

		res, err := subject.finish(ctx, FinishRequest{
			ImportPath:      importPath,
			FinishToken:     start.FinishToken,
			ArchivePath:     archive,
			AdditionalFiles: map[Label]string{label: extraFile},
		})
		require.ErrorContains(t, err, extraFile)
		require.Nil(t, res)
	})
}
