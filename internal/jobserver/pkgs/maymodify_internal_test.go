// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package pkgs

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/DataDog/orchestrion/internal/jobserver/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/tools/go/packages"
)

func TestMayModify(t *testing.T) {
	aspectYML := func(joinPoint string) string {
		return "meta:\n  name: test\n  description: test\naspects:\n  - id: test\n    join-point:\n" + joinPoint +
			"    advice:\n      - prepend-statements:\n          template: _ = 0\n"
	}

	dir := t.TempDir()
	writeFile := func(dir string, name string, content string) {
		t.Helper()
		path := filepath.Join(dir, filepath.FromSlash(name))
		require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
		require.NoError(t, os.WriteFile(path, []byte(content), 0o644))
	}
	writeFile(dir, "go.mod", "module example.com/gate\n\ngo 1.25\n")
	writeFile(dir, "orchestrion.tool.go", "//go:build tools\n\npackage tools\n\nimport _ \"example.com/gate/instrumentation\"\n")
	writeFile(dir, "instrumentation/instrumentation.go", "package instrumentation\n")
	writeFile(dir, "instrumentation/orchestrion.yml", aspectYML("      directive: gate:marker\n"))
	writeFile(dir, "subject/subject.go", "package subject\n\n//other:marker\nvar V int\n")

	ctx := context.Background()
	s := &service{loaded: common.NewCache[*packages.Package](&common.CacheStats{})}

	req := MayModifyRequest{
		ConfigDir:  dir,
		WorkDir:    dir,
		ImportPath: "example.com/gate/subject",
		Files:      []string{filepath.Join(dir, "subject", "subject.go")},
	}
	mayModify := func(req MayModifyRequest) bool {
		t.Helper()
		res, err := s.mayModify(ctx, req)
		require.NoError(t, err)
		return !res.NoAspectMayApply
	}

	assert.False(t, mayModify(req), "the only aspect cannot match on subject.go")

	// Changing a configuration file's content invalidates the cached configuration...
	writeFile(dir, "instrumentation/orchestrion.yml", aspectYML("      directive: other:marker\n"))
	assert.True(t, mayModify(req), "the updated aspect may match on subject.go")
	// ... and so does reverting it.
	writeFile(dir, "instrumentation/orchestrion.yml", aspectYML("      directive: gate:marker\n"))
	assert.False(t, mayModify(req), "the reverted aspect cannot match on subject.go")

	// Creating a configuration file that was looked for (but was absent) invalidates it too.
	writeFile(dir, "orchestrion.yml", aspectYML("      package-filter:\n        root: true\n"))
	assert.True(t, mayModify(req), "the new aspect applies to all packages of the root module")

	// The root module is determined from the request's working directory.
	other := t.TempDir()
	writeFile(other, "go.mod", "module example.com/other\n\ngo 1.25\n")
	writeFile(other, "main.go", "package main\n")
	req.WorkDir = other
	assert.False(t, mayModify(req), "example.com/gate/subject is not part of the root module")
}

func TestMayModifyResponseDefaultsToMayApply(t *testing.T) {
	// A response without a result (which a job server never sends on purpose) must not cause the package
	// to be skipped.
	for _, data := range []string{`{}`, `{"result":null}`, `{"error":""}`} {
		res, err := common.UnmarshalResponse[MayModifyResponse](context.Background(), []byte(data))
		require.NoError(t, err, data)
		assert.False(t, res.NoAspectMayApply, data)
	}

	res, err := common.UnmarshalResponse[MayModifyResponse](context.Background(), []byte(`{"result":{"noAspectMayApply":true}}`))
	require.NoError(t, err)
	assert.True(t, res.NoAspectMayApply)
}
