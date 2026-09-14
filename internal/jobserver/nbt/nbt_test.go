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
	"time"

	"github.com/DataDog/orchestrion/internal/jobserver/common"
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
		res, err := subject.finish(ctx, FinishRequest{ImportPath: importPath, BuildID: buildID, FinishToken: "bazinga"})
		require.ErrorContains(t, err, "no build started")
		require.Nil(t, res)
	})

	t.Run("start-reuse-finish", func(t *testing.T) {
		subject := &service{dir: t.TempDir()}

		start, err := subject.start(ctx, StartRequest{ImportPath: importPath, BuildID: buildID})
		require.NoError(t, err)
		require.NotEmpty(t, start.FinishToken)
		assert.Empty(t, start.Files)

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
				assert.NotEmpty(t, res.Files)
				assert.Len(t, res.Files, 2)

				path, ok := res.Files[LabelArchive]
				assert.True(t, ok, "no file returned for %s", LabelArchive)
				content, err := os.ReadFile(path)
				assert.NoError(t, err)
				assert.Equal(t, archiveContent, string(content))

				path, ok = res.Files[label]
				assert.True(t, ok, "no file returned for %s", label)
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
			ImportPath:  importPath,
			BuildID:     buildID,
			FinishToken: start.FinishToken,
			Files:       map[Label]string{LabelArchive: archive, label: extraFile},
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
		assert.Empty(t, start.Files)

		archiveContent := uuid.NewString()
		archive := filepath.Join(t.TempDir(), "_pkg_.a")
		require.NoError(t, os.WriteFile(archive, []byte(archiveContent), 0o644))

		for range 10 {
			res, err := subject.finish(ctx, FinishRequest{
				ImportPath:  importPath,
				BuildID:     buildID,
				FinishToken: start.FinishToken,
				Files:       map[Label]string{LabelArchive: archive},
			})
			require.NoError(t, err)
			require.NotNil(t, res)
		}
	})

	t.Run("start-different-buildid", func(t *testing.T) {
		// This reproduces https://github.com/DataDog/orchestrion/issues/653
		// Test for PGO support: same importPath with different buildIDs should compile independently
		const importPath = "github.com/DataDog/orchestrion.test"
		subject := &service{dir: t.TempDir()}

		// Start compilation with first build ID (e.g., without PGO)
		buildID1 := uuid.NewString()
		start1, err := subject.start(ctx, StartRequest{ImportPath: importPath, BuildID: buildID1})
		require.NoError(t, err)
		require.NotEmpty(t, start1.FinishToken, "First start should get a finish token")
		assert.Empty(t, start1.Files)

		// Start compilation with second build ID (e.g., with PGO enabled)
		// This should NOT error - it should get its own finish token
		buildID2 := uuid.NewString()
		start2, err := subject.start(ctx, StartRequest{ImportPath: importPath, BuildID: buildID2})
		require.NoError(t, err, "Second start with different buildID should succeed")
		require.NotEmpty(t, start2.FinishToken, "Second start should get a finish token")
		assert.Empty(t, start2.Files)
		assert.NotEqual(t, start1.FinishToken, start2.FinishToken, "Different build IDs should get different tokens")

		// Finish first compilation
		archive1Content := uuid.NewString()
		archive1 := filepath.Join(t.TempDir(), "archive1.a")
		require.NoError(t, os.WriteFile(archive1, []byte(archive1Content), 0o644))

		res1, err := subject.finish(ctx, FinishRequest{
			ImportPath:  importPath,
			BuildID:     buildID1,
			FinishToken: start1.FinishToken,
			Files:       map[Label]string{LabelArchive: archive1},
		})
		require.NoError(t, err)
		require.NotNil(t, res1)

		// Finish second compilation independently
		archive2Content := uuid.NewString()
		archive2 := filepath.Join(t.TempDir(), "archive2.a")
		require.NoError(t, os.WriteFile(archive2, []byte(archive2Content), 0o644))

		res2, err := subject.finish(ctx, FinishRequest{
			ImportPath:  importPath,
			BuildID:     buildID2,
			FinishToken: start2.FinishToken,
			Files:       map[Label]string{LabelArchive: archive2},
		})
		require.NoError(t, err)
		require.NotNil(t, res2)

		// Verify concurrent requests for each specific buildID still get cached results
		var wg sync.WaitGroup
		defer wg.Wait()

		// Concurrent requests for buildID1 should reuse its artifacts
		for range 5 {
			wg.Add(1)
			go func() {
				defer wg.Done()

				res, err := subject.start(ctx, StartRequest{ImportPath: importPath, BuildID: buildID1})
				assert.NoError(t, err)
				assert.Empty(t, res.FinishToken)
				assert.NotEmpty(t, res.Files)

				path, ok := res.Files[LabelArchive]
				assert.True(t, ok)
				content, err := os.ReadFile(path)
				assert.NoError(t, err)
				assert.Equal(t, archive1Content, string(content))
			}()
		}

		// Concurrent requests for buildID2 should reuse its artifacts
		for range 5 {
			wg.Add(1)
			go func() {
				defer wg.Done()

				res, err := subject.start(ctx, StartRequest{ImportPath: importPath, BuildID: buildID2})
				assert.NoError(t, err)
				assert.Empty(t, res.FinishToken)
				assert.NotEmpty(t, res.Files)

				path, ok := res.Files[LabelArchive]
				assert.True(t, ok)
				content, err := os.ReadFile(path)
				assert.NoError(t, err)
				assert.Equal(t, archive2Content, string(content))
			}()
		}
	})

	t.Run("start-badtoken-finish", func(t *testing.T) {
		const importPath = "github.com/DataDog/orchestrion.test"
		subject := &service{dir: t.TempDir()}

		start, err := subject.start(ctx, StartRequest{ImportPath: importPath, BuildID: buildID})
		require.NoError(t, err)
		require.NotEmpty(t, start.FinishToken)
		assert.Empty(t, start.Files)

		archiveContent := uuid.NewString()
		archive := filepath.Join(t.TempDir(), "_pkg_.a")
		require.NoError(t, os.WriteFile(archive, []byte(archiveContent), 0o644))

		for range 10 {
			res, err := subject.finish(ctx, FinishRequest{
				ImportPath:  importPath,
				BuildID:     buildID,
				FinishToken: uuid.NewString(),
				Files:       map[Label]string{LabelArchive: archive},
			})
			require.Error(t, err, "invalid finish token")
			require.Nil(t, res)
		}

		res, err := subject.finish(ctx, FinishRequest{
			ImportPath:  importPath,
			BuildID:     buildID,
			FinishToken: start.FinishToken,
			Files:       map[Label]string{LabelArchive: archive},
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
		assert.Empty(t, start.Files)

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
			BuildID:     buildID,
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
		assert.Empty(t, start.Files)

		var wg sync.WaitGroup
		defer wg.Wait()
		for range 10 {
			wg.Add(1)
			go func() {
				defer wg.Done()

				res, err := subject.start(ctx, StartRequest{ImportPath: importPath, BuildID: buildID})
				assert.ErrorContains(t, err, errNoFilesNorError.Error())
				assert.Nil(t, res)
			}()
		}

		res, err := subject.finish(ctx, FinishRequest{
			ImportPath:  importPath,
			BuildID:     buildID,
			FinishToken: start.FinishToken,
		})
		require.ErrorIs(t, err, errNoFilesNorError)
		require.Nil(t, res)
	})

	t.Run("start-reuse-missing.archive.file", func(t *testing.T) {
		const importPath = "github.com/DataDog/orchestrion.test"
		subject := &service{dir: t.TempDir()}

		start, err := subject.start(ctx, StartRequest{ImportPath: importPath, BuildID: buildID})
		require.NoError(t, err)
		require.NotEmpty(t, start.FinishToken)
		assert.Empty(t, start.Files)

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
			BuildID:     buildID,
			FinishToken: start.FinishToken,
			Files:       map[Label]string{LabelArchive: archive},
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
		assert.Empty(t, start.Files)

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
			ImportPath:  importPath,
			BuildID:     buildID,
			FinishToken: start.FinishToken,
			Files:       map[Label]string{LabelArchive: archive, label: extraFile},
		})
		require.ErrorContains(t, err, extraFile)
		require.Nil(t, res)
	})

	// Compilation tasks may spawn nested builds in order to resolve the archives
	// of the dependencies injected code introduced. If such a nested build ends up
	// compiling a package that is already being built by one of its ancestors, the
	// artifacts of that task can never become available, and waiting for them
	// deadlocks the whole build.
	t.Run("re-entrant", func(t *testing.T) {
		const nestedImportPath = "github.com/DataDog/orchestrion.test/nested"

		t.Run("self", func(t *testing.T) {
			subject := &service{dir: t.TempDir(), graph: &common.Graph{}}

			start, err := subject.start(ctx, StartRequest{ImportPath: importPath, BuildID: buildID})
			require.NoError(t, err)
			require.NotEmpty(t, start.FinishToken)

			res, err := startWithin(t, subject, StartRequest{
				ImportPath:       importPath,
				BuildID:          buildID,
				ParentImportPath: importPath,
			})
			require.ErrorContains(t, err, "cycle detected: "+importPath+" -> "+importPath)
			require.Nil(t, res)
		})

		t.Run("transitive", func(t *testing.T) {
			graph := &common.Graph{}
			subject := &service{dir: t.TempDir(), graph: graph}

			start, err := subject.start(ctx, StartRequest{ImportPath: importPath, BuildID: buildID})
			require.NoError(t, err)
			require.NotEmpty(t, start.FinishToken)

			// The in-flight compilation of importPath is resolving dependencies through
			// a nested build that is compiling nestedImportPath.
			require.NoError(t, graph.AddEdge(importPath, nestedImportPath))

			res, err := startWithin(t, subject, StartRequest{
				ImportPath:       importPath,
				BuildID:          buildID,
				ParentImportPath: nestedImportPath,
			})
			require.ErrorContains(t, err, "cycle detected: "+importPath+" -> "+nestedImportPath+" -> "+importPath)
			require.Nil(t, res)
		})

		t.Run("no-cycle", func(t *testing.T) {
			graph := &common.Graph{}
			subject := &service{dir: t.TempDir(), graph: graph}

			start, err := subject.start(ctx, StartRequest{ImportPath: importPath, BuildID: buildID})
			require.NoError(t, err)
			require.NotEmpty(t, start.FinishToken)

			archiveContent := uuid.NewString()
			archive := filepath.Join(t.TempDir(), "_pkg_.a")
			require.NoError(t, os.WriteFile(archive, []byte(archiveContent), 0o644))

			var wg sync.WaitGroup
			wg.Add(1)
			go func() {
				defer wg.Done()

				res, err := subject.start(ctx, StartRequest{
					ImportPath:       importPath,
					BuildID:          buildID,
					ParentImportPath: nestedImportPath,
				})
				if !assert.NoError(t, err) {
					return
				}
				assert.Empty(t, res.FinishToken)
				content, err := os.ReadFile(res.Files[LabelArchive])
				assert.NoError(t, err)
				assert.Equal(t, archiveContent, string(content))
			}()

			res, err := subject.finish(ctx, FinishRequest{
				ImportPath:  importPath,
				BuildID:     buildID,
				FinishToken: start.FinishToken,
				Files:       map[Label]string{LabelArchive: archive},
			})
			require.NoError(t, err)
			require.NotNil(t, res)
			wg.Wait()

			// The wait was released, so the reverse edge no longer closes a cycle.
			require.NoError(t, graph.AddEdge(importPath, nestedImportPath))
		})
	})
}

// startWithin calls [service.start] and fails the test if it blocks instead of
// returning, which is what a missing cyclic wait detection results in.
func startWithin(t *testing.T, subject *service, req StartRequest) (*StartResponse, error) {
	t.Helper()

	type result struct {
		res *StartResponse
		err error
	}
	done := make(chan result, 1)
	go func() {
		res, err := subject.start(context.Background(), req)
		done <- result{res, err}
	}()

	select {
	case res := <-done:
		return res.res, res.err
	case <-time.After(10 * time.Second):
		t.Fatalf("waiting for the concurrent compilation of %q did not return", req.ImportPath)
		return nil, nil
	}
}
