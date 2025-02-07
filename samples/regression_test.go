// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package samples_test

import (
	"bytes"
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/DataDog/orchestrion/internal/injector"
	"github.com/DataDog/orchestrion/internal/injector/aspect"
	"github.com/DataDog/orchestrion/internal/injector/config"
	"github.com/DataDog/orchestrion/internal/version"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/tools/go/packages"
)

var (
	samplesDir      string
	referenceDir    string
	updateSnapshots bool
)

func TestSamples(t *testing.T) {
	t.Parallel()

	config, err := config.NewLoader(samplesDir, true).Load()
	require.NoError(t, err)
	aspects := config.Aspects()

	dirs, err := os.ReadDir(samplesDir)
	require.NoError(t, err)
	for _, dir := range dirs {
		if !dir.IsDir() || dir.Name() == "testdata" {
			continue
		}

		dir := dir.Name()
		t.Run(dir, func(t *testing.T) {
			t.Parallel()

			pkgDir := filepath.Join(samplesDir, dir)

			testLookup := func(path string) (io.ReadCloser, error) {
				pkgs, err := packages.Load(
					&packages.Config{Mode: packages.NeedExportFile, Dir: pkgDir},
					path,
				)
				if err != nil {
					return nil, err
				}
				file := pkgs[0].ExportFile
				if file == "" {
					return nil, fmt.Errorf("no export data for %s", path)
				}
				return os.Open(file)
			}

			importMap := map[string]string{}
			for _, a := range aspects {
				for _, i := range a.JoinPoint.ImpliesImported() {
					importMap[i] = ""
				}
			}

			tmp := t.TempDir()
			inj := injector.Injector{
				ModifiedFile: func(filename string) string {
					return filepath.Join(tmp, filepath.Base(filename))
				},
				RootConfig: map[string]string{"httpmode": "wrap"},
				ImportPath: fmt.Sprintf("github.com/DataDog/orchestrion/samples/%s", dir),
				Lookup:     testLookup,
				ImportMap:  importMap,
			}

			files, err := filepath.Glob(filepath.Join(pkgDir, "*.go"))
			require.NoError(t, err)

			copyAspects := make([]*aspect.Aspect, len(aspects))
			copy(copyAspects, aspects)

			results, _, err := inj.InjectFiles(context.Background(), files, copyAspects)
			require.NoError(t, err)

			for _, filename := range files {
				referenceFile := filepath.Join(referenceDir, dir, filepath.Base(filename)) + ".snap"

				res, modified := results[filename]
				if !modified {
					_, err := os.Stat(referenceFile)
					if updateSnapshots && err == nil {
						require.NoError(t, os.Remove(referenceFile))
					}
					require.ErrorIs(t, err, os.ErrNotExist, "expected no snapshot to exist for %s", filename)
					return
				}

				data, err := os.ReadFile(res.Filename)
				require.NoError(t, err)

				data = bytes.ReplaceAll(data, []byte(samplesDir), []byte("samples"))
				data = bytes.ReplaceAll(data, []byte(fmt.Sprintf("%q", version.Tag())), []byte("\"<version.Tag>\""))
				// Normalize CRLF back to LF so Windows behaves the same as Unix.
				data = bytes.ReplaceAll(data, []byte{'\r', '\n'}, []byte{'\n'})

				reference, err := os.ReadFile(referenceFile)
				if updateSnapshots && errors.Is(err, os.ErrNotExist) {
					require.NoError(t, os.MkdirAll(filepath.Dir(referenceFile), 0o755))
					require.NoError(t, os.WriteFile(referenceFile, data, 0o644))
				}
				require.NoError(t, err)

				if !assert.Equal(t, string(reference), string(data)) && updateSnapshots {
					require.NoError(t, os.WriteFile(referenceFile, data, 0o644))
				}
			}
		})
	}
}

func init() {
	_, filename, _, _ := runtime.Caller(0)
	samplesDir = filepath.Join(filename, "..")
	referenceDir = filepath.Join(samplesDir, "testdata")

	flag.BoolVar(&updateSnapshots, "update", os.Getenv("UPDATE_SNAPSHOTS") != "", "update snapshots")
}
