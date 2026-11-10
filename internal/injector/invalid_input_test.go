// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package injector_test

import (
	gocontext "context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/DataDog/orchestrion/internal/injector"
	"github.com/stretchr/testify/require"
	"golang.org/x/tools/go/packages"
)

// TestSkipsInvalidOriginalImportNames (D07) covers original programs where an import alias
// duplicates a package-level declaration, in the same file or in another file of the package.
// These programs are invalid Go before any injection, so the injector must keep its existing
// rejection behavior: skip the package without error and surface nothing to weave.
func TestSkipsInvalidOriginalImportNames(t *testing.T) {
	t.Parallel()

	for name, files := range map[string][]struct{ name, code string }{
		"same-file": {{
			"input.go", `package test

import str "strings"

var str = 1
`,
		}},
		"other-file": {
			{
				"input.go", `package test

import str "strings"

var _ = str.ToLower
`,
			},
			{"extra.go", "package test\n\nvar str = 1\n"},
		},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			tmp := t.TempDir()
			cmd := exec.Command("go", "mod", "init", "dummy/test/module")
			cmd.Dir = tmp
			require.NoError(t, cmd.Run())

			paths := make([]string, 0, len(files))
			for _, file := range files {
				path := filepath.Join(tmp, file.name)
				require.NoError(t, os.WriteFile(path, []byte(file.code), 0o644))
				paths = append(paths, path)
			}

			lookup := func(path string) (io.ReadCloser, error) {
				pkgs, err := packages.Load(
					&packages.Config{Mode: packages.NeedExportFile, Dir: tmp},
					path,
				)
				if err != nil {
					return nil, err
				}
				if len(pkgs) == 0 || pkgs[0].ExportFile == "" {
					return nil, fmt.Errorf("no export file found for %q", path)
				}
				return os.Open(pkgs[0].ExportFile)
			}

			inj := injector.Injector{
				ImportPath: "dummy/test/module",
				Lookup:     lookup,
				ImportMap:  map[string]string{"strings": ""},
			}

			res, _, err := inj.InjectFiles(gocontext.Background(), paths, nil)
			require.NoError(t, err, "invalid originals must be skipped, not failed: %s", strings.Join(paths, ", "))
			require.Empty(t, res, "nothing may be injected into an invalid original package")
		})
	}
}
