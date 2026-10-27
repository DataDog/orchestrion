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

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func testDifferentBuildID(t *testing.T, testContext serviceTestContext) {
	t.Run("start-different-buildid", func(t *testing.T) {
		// This reproduces https://github.com/DataDog/orchestrion/issues/653:
		// the same import path with different build IDs must compile independently for PGO.
		subject := &service{dir: t.TempDir()}

		buildID1 := uuid.NewString()
		start1, err := subject.start(testContext.ctx, StartRequest{ImportPath: testContext.importPath, BuildID: buildID1})
		require.NoError(t, err)
		require.NotEmpty(t, start1.FinishToken, "First start should get a finish token")
		assert.Empty(t, start1.Files)

		buildID2 := uuid.NewString()
		start2, err := subject.start(testContext.ctx, StartRequest{ImportPath: testContext.importPath, BuildID: buildID2})
		require.NoError(t, err, "Second start with different buildID should succeed")
		require.NotEmpty(t, start2.FinishToken, "Second start should get a finish token")
		assert.Empty(t, start2.Files)
		assert.NotEqual(t, start1.FinishToken, start2.FinishToken, "Different build IDs should get different tokens")

		archive1Content := uuid.NewString()
		archive1 := filepath.Join(t.TempDir(), "archive1.a")
		require.NoError(t, os.WriteFile(archive1, []byte(archive1Content), 0o644))

		res1, err := subject.finish(testContext.ctx, FinishRequest{
			ImportPath:  testContext.importPath,
			BuildID:     buildID1,
			FinishToken: start1.FinishToken,
			Files:       map[Label]string{LabelArchive: archive1},
		})
		require.NoError(t, err)
		require.NotNil(t, res1)

		archive2Content := uuid.NewString()
		archive2 := filepath.Join(t.TempDir(), "archive2.a")
		require.NoError(t, os.WriteFile(archive2, []byte(archive2Content), 0o644))

		res2, err := subject.finish(testContext.ctx, FinishRequest{
			ImportPath:  testContext.importPath,
			BuildID:     buildID2,
			FinishToken: start2.FinishToken,
			Files:       map[Label]string{LabelArchive: archive2},
		})
		require.NoError(t, err)
		require.NotNil(t, res2)

		var wg sync.WaitGroup
		defer wg.Wait()

		for range 5 {
			wg.Add(1)
			go func() {
				defer wg.Done()

				res, err := subject.start(testContext.ctx, StartRequest{ImportPath: testContext.importPath, BuildID: buildID1})
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

		for range 5 {
			wg.Add(1)
			go func() {
				defer wg.Done()

				res, err := subject.start(testContext.ctx, StartRequest{ImportPath: testContext.importPath, BuildID: buildID2})
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
}

func testReuseError(t *testing.T, testContext serviceTestContext) {
	t.Run("start-reuse-error", func(t *testing.T) {
		subject := &service{dir: t.TempDir()}

		start, err := subject.start(testContext.ctx, StartRequest{ImportPath: testContext.importPath, BuildID: testContext.buildID})
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

				res, err := subject.start(testContext.ctx, StartRequest{ImportPath: testContext.importPath, BuildID: testContext.buildID})
				assert.ErrorContains(t, err, errorText)
				assert.Nil(t, res)
			}()
		}

		res, err := subject.finish(testContext.ctx, FinishRequest{
			ImportPath:  testContext.importPath,
			BuildID:     testContext.buildID,
			FinishToken: start.FinishToken,
			Error:       &errorText,
		})
		require.NoError(t, err)
		require.NotNil(t, res)
	})
}

func testReuseBadResponse(t *testing.T, testContext serviceTestContext) {
	t.Run("start-reuse-bad-response", func(t *testing.T) {
		subject := &service{dir: t.TempDir()}

		start, err := subject.start(testContext.ctx, StartRequest{ImportPath: testContext.importPath, BuildID: testContext.buildID})
		require.NoError(t, err)
		require.NotEmpty(t, start.FinishToken)
		assert.Empty(t, start.Files)

		var wg sync.WaitGroup
		defer wg.Wait()
		for range 10 {
			wg.Add(1)
			go func() {
				defer wg.Done()

				res, err := subject.start(testContext.ctx, StartRequest{ImportPath: testContext.importPath, BuildID: testContext.buildID})
				assert.ErrorContains(t, err, errNoFilesNorError.Error())
				assert.Nil(t, res)
			}()
		}

		res, err := subject.finish(testContext.ctx, FinishRequest{
			ImportPath:  testContext.importPath,
			BuildID:     testContext.buildID,
			FinishToken: start.FinishToken,
		})
		require.ErrorIs(t, err, errNoFilesNorError)
		require.Nil(t, res)
	})
}

func testReuseMissingArchive(t *testing.T, testContext serviceTestContext) {
	t.Run("start-reuse-missing.archive.file", func(t *testing.T) {
		subject := &service{dir: t.TempDir()}

		start, err := subject.start(testContext.ctx, StartRequest{ImportPath: testContext.importPath, BuildID: testContext.buildID})
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

				res, err := subject.start(testContext.ctx, StartRequest{ImportPath: testContext.importPath, BuildID: testContext.buildID})
				assert.ErrorContains(t, err, archive)
				assert.Nil(t, res)
			}()
		}

		res, err := subject.finish(testContext.ctx, FinishRequest{
			ImportPath:  testContext.importPath,
			BuildID:     testContext.buildID,
			FinishToken: start.FinishToken,
			Files:       map[Label]string{LabelArchive: archive},
		})
		require.ErrorContains(t, err, archive)
		require.Nil(t, res)
	})
}

func testReuseMissingExtraFile(t *testing.T, testContext serviceTestContext) {
	t.Run("start-reuse-missing.extra.file", func(t *testing.T) {
		subject := &service{dir: t.TempDir()}

		start, err := subject.start(testContext.ctx, StartRequest{ImportPath: testContext.importPath, BuildID: testContext.buildID})
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

				res, err := subject.start(testContext.ctx, StartRequest{ImportPath: testContext.importPath, BuildID: testContext.buildID})
				assert.ErrorContains(t, err, extraFile)
				assert.Nil(t, res)
			}()
		}

		archive := filepath.Join(t.TempDir(), "_pkg_.a")
		require.NoError(t, os.WriteFile(archive, []byte(uuid.NewString()), 0o644))

		res, err := subject.finish(testContext.ctx, FinishRequest{
			ImportPath:  testContext.importPath,
			BuildID:     testContext.buildID,
			FinishToken: start.FinishToken,
			Files:       map[Label]string{LabelArchive: archive, label: extraFile},
		})
		require.ErrorContains(t, err, extraFile)
		require.Nil(t, res)
	})
}

func testReentrantStart(t *testing.T, testContext serviceTestContext) {
	// Compilation tasks resolve the archives of the dependencies injected code
	// introduced by spawning nested builds, which may have to compile a package one
	// of their ancestors is already compiling. The artifacts of that task can only
	// be produced once the nested build completed, so waiting for them deadlocks
	// the whole build. See TestCompileLoop in `test/e2e` for the same scenario in a
	// real build.
	t.Run("re-entrant", func(t *testing.T) {
		subject := &service{dir: t.TempDir()}

		start, err := subject.start(testContext.ctx, StartRequest{ImportPath: testContext.importPath, BuildID: testContext.buildID})
		require.NoError(t, err)
		require.NotEmpty(t, start.FinishToken)

		// The same package, compiled by a nested build the in-flight task spawned.
		// That task cannot report its outcome until this request has returned.
		type result struct {
			res *StartResponse
			err error
		}
		done := make(chan result, 1)
		go func() {
			res, err := subject.start(context.Background(), StartRequest{
				ImportPath:       testContext.importPath,
				BuildID:          testContext.buildID,
				ParentImportPath: testContext.importPath,
			})
			done <- result{res, err}
		}()

		select {
		case got := <-done:
			require.ErrorContains(t, got.err, "cycle detected")
			require.Nil(t, got.res)
		case <-time.After(5 * time.Second):
			t.Fatalf("start() is still waiting for the compilation of %q, which cannot complete until it returns", testContext.importPath)
		}
	})
}
