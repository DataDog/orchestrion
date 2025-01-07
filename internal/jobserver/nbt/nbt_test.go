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

	t.Run("not-started", func(t *testing.T) {
		subject := &service{dir: t.TempDir()}
		res, err := subject.finish(ctx, FinishRequest{})
		require.ErrorContains(t, err, "no build started")
		require.Nil(t, res)
	})

	t.Run("start-reuse-finish", func(t *testing.T) {
		const importPath = "github.com/DataDog/orchestrion.test"
		subject := &service{dir: t.TempDir()}

		start, err := subject.start(ctx, StartRequest{ImportPath: importPath})
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

				res, err := subject.start(ctx, StartRequest{ImportPath: importPath})
				assert.NoError(t, err)
				assert.Empty(t, res.FinishToken)
				assert.NotEmpty(t, res.ArchivePath)

				content, err := os.ReadFile(res.ArchivePath)
				assert.NoError(t, err)
				assert.Equal(t, archiveContent, string(content))
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

		start, err := subject.start(ctx, StartRequest{ImportPath: importPath})
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

	t.Run("start-reuse-error", func(t *testing.T) {
		const importPath = "github.com/DataDog/orchestrion.test"
		subject := &service{dir: t.TempDir()}

		start, err := subject.start(ctx, StartRequest{ImportPath: importPath})
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

				res, err := subject.start(ctx, StartRequest{ImportPath: importPath})
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

		start, err := subject.start(ctx, StartRequest{ImportPath: importPath})
		require.NoError(t, err)
		require.NotEmpty(t, start.FinishToken)
		assert.Empty(t, start.ArchivePath)

		var wg sync.WaitGroup
		defer wg.Wait()
		for range 10 {
			wg.Add(1)
			go func() {
				defer wg.Done()

				res, err := subject.start(ctx, StartRequest{ImportPath: importPath})
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

	t.Run("start-reuse-fail", func(t *testing.T) {
		const importPath = "github.com/DataDog/orchestrion.test"
		subject := &service{dir: t.TempDir()}

		start, err := subject.start(ctx, StartRequest{ImportPath: importPath})
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

				res, err := subject.start(ctx, StartRequest{ImportPath: importPath})
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
}
