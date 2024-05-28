// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package builtin_test

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/datadog/orchestrion/internal/injector"
	"github.com/datadog/orchestrion/internal/injector/builtin"
	"github.com/datadog/orchestrion/internal/version"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var (
	samplesDir      string
	referenceDir    string
	updateSnapshots bool
)

func Test(t *testing.T) {
	t.Parallel()

	dirs, err := os.ReadDir(samplesDir)
	require.NoError(t, err)
	for _, dir := range dirs {
		if !dir.IsDir() {
			continue
		}

		dir := dir.Name()
		t.Run(dir, func(t *testing.T) {
			t.Parallel()

			pkgDir := filepath.Join(samplesDir, dir)

			tmp := t.TempDir()
			inj, err := injector.New(pkgDir, injector.Options{
				Aspects: builtin.Aspects[:],
				Dir:     pkgDir,
				ModifiedFile: func(filename string) string {
					return filepath.Join(tmp, filepath.Base(filename))
				},
				PreserveLineInfo: true,
			})
			require.NoError(t, err)

			files, err := filepath.Glob(filepath.Join(pkgDir, "*.go"))
			require.NoError(t, err)
			for _, filename := range files {
				t.Run(filepath.Base(filename), func(t *testing.T) {
					res, err := inj.InjectFile(filename, map[string]string{"httpmode": "wrap"})
					require.NoError(t, err)

					referenceFile := filepath.Join(referenceDir, dir, filepath.Base(filename)) + ".snap"
					if !res.Modified {
						_, err := os.Stat(referenceFile)
						if updateSnapshots && err == nil {
							require.NoError(t, os.Remove(referenceFile))
						}
						require.ErrorIs(t, err, os.ErrNotExist)
						return
					}

					data, err := os.ReadFile(res.Filename)
					require.NoError(t, err)

					data = bytes.ReplaceAll(data, []byte(samplesDir), []byte("samples"))
					data = bytes.ReplaceAll(data, []byte(fmt.Sprintf("%q", version.Tag)), []byte("\"<version.Tag>\""))

					reference, err := os.ReadFile(referenceFile)
					if updateSnapshots && errors.Is(err, os.ErrNotExist) {
						require.NoError(t, os.MkdirAll(filepath.Dir(referenceFile), 0o755))
						require.NoError(t, os.WriteFile(referenceFile, data, 0o644))
					}
					require.NoError(t, err)

					if !assert.Equal(t, string(reference), string(data)) && updateSnapshots {
						require.NoError(t, os.WriteFile(referenceFile, data, 0o644))
					}
				})
			}
		})
	}
}

func init() {
	_, filename, _, _ := runtime.Caller(0)
	samplesDir = filepath.Join(filepath.Dir(filepath.Dir(filepath.Dir(filepath.Dir(filename)))), "samples")
	referenceDir = filepath.Join(filepath.Dir(filename), "testdata")

	updateSnapshots = os.Getenv("UPDATE_SNAPSHOTS") != ""
}
