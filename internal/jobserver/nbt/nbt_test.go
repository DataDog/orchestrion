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

type serviceTestContext struct {
	ctx        context.Context
	importPath string
	buildID    string
}

func Test(t *testing.T) {
	ctx := context.Background()
	if deadline, ok := t.Deadline(); ok {
		var cancel func()
		ctx, cancel = context.WithDeadline(context.Background(), deadline)
		defer cancel()
	}

	testContext := serviceTestContext{
		ctx:        ctx,
		importPath: "github.com/DataDog/orchestrion.test",
		buildID:    uuid.NewString(),
	}

	testFinishWithoutStart(t, testContext)
	testStartReuseFinish(t, testContext)
	testRepeatedFinish(t, testContext)
	testDifferentBuildID(t, testContext)
	testBadFinishToken(t, testContext)
	testReuseError(t, testContext)
	testReuseBadResponse(t, testContext)
	testReuseMissingArchive(t, testContext)
	testReuseMissingExtraFile(t, testContext)
	testReentrantStart(t, testContext)
}

func testFinishWithoutStart(t *testing.T, testContext serviceTestContext) {
	t.Run("not-started", func(t *testing.T) {
		subject := &service{dir: t.TempDir()}
		res, err := subject.finish(testContext.ctx, FinishRequest{
			ImportPath:  testContext.importPath,
			BuildID:     testContext.buildID,
			FinishToken: "bazinga",
		})
		require.ErrorContains(t, err, "no build started")
		require.Nil(t, res)
	})
}

func testStartReuseFinish(t *testing.T, testContext serviceTestContext) {
	t.Run("start-reuse-finish", func(t *testing.T) {
		subject := &service{dir: t.TempDir()}

		start, err := subject.start(testContext.ctx, StartRequest{ImportPath: testContext.importPath, BuildID: testContext.buildID})
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

				res, err := subject.start(testContext.ctx, StartRequest{ImportPath: testContext.importPath, BuildID: testContext.buildID})
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

		res, err := subject.finish(testContext.ctx, FinishRequest{
			ImportPath:  testContext.importPath,
			BuildID:     testContext.buildID,
			FinishToken: start.FinishToken,
			Files:       map[Label]string{LabelArchive: archive, label: extraFile},
		})
		require.NoError(t, err)
		require.NotNil(t, res)
	})
}

func testRepeatedFinish(t *testing.T, testContext serviceTestContext) {
	t.Run("start-finish-finish", func(t *testing.T) {
		subject := &service{dir: t.TempDir()}

		start, err := subject.start(testContext.ctx, StartRequest{ImportPath: testContext.importPath, BuildID: testContext.buildID})
		require.NoError(t, err)
		require.NotEmpty(t, start.FinishToken)
		assert.Empty(t, start.Files)

		archiveContent := uuid.NewString()
		archive := filepath.Join(t.TempDir(), "_pkg_.a")
		require.NoError(t, os.WriteFile(archive, []byte(archiveContent), 0o644))

		for range 10 {
			res, err := subject.finish(testContext.ctx, FinishRequest{
				ImportPath:  testContext.importPath,
				BuildID:     testContext.buildID,
				FinishToken: start.FinishToken,
				Files:       map[Label]string{LabelArchive: archive},
			})
			require.NoError(t, err)
			require.NotNil(t, res)
		}
	})
}

func testBadFinishToken(t *testing.T, testContext serviceTestContext) {
	t.Run("start-badtoken-finish", func(t *testing.T) {
		subject := &service{dir: t.TempDir()}

		start, err := subject.start(testContext.ctx, StartRequest{ImportPath: testContext.importPath, BuildID: testContext.buildID})
		require.NoError(t, err)
		require.NotEmpty(t, start.FinishToken)
		assert.Empty(t, start.Files)

		archiveContent := uuid.NewString()
		archive := filepath.Join(t.TempDir(), "_pkg_.a")
		require.NoError(t, os.WriteFile(archive, []byte(archiveContent), 0o644))

		for range 10 {
			res, err := subject.finish(testContext.ctx, FinishRequest{
				ImportPath:  testContext.importPath,
				BuildID:     testContext.buildID,
				FinishToken: uuid.NewString(),
				Files:       map[Label]string{LabelArchive: archive},
			})
			require.Error(t, err, "invalid finish token")
			require.Nil(t, res)
		}

		res, err := subject.finish(testContext.ctx, FinishRequest{
			ImportPath:  testContext.importPath,
			BuildID:     testContext.buildID,
			FinishToken: start.FinishToken,
			Files:       map[Label]string{LabelArchive: archive},
		})
		require.NoError(t, err)
		require.NotNil(t, res)
	})
}
