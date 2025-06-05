// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package report

import (
	"context"
	"io/fs"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/DataDog/orchestrion/internal/toolexec/aspect"
	"github.com/liamg/memoryfs"
)

func TestFromWorkFS(t *testing.T) {
	tests := []struct {
		name    string
		args    fs.FS
		want    []ModifiedFile
		wantErr bool
	}{
		{
			name: "empty",
			args: func() fs.FS {
				fsys := memoryfs.New()
				return fsys
			}(),
		},
		{
			name: "no-line-directive",
			args: func() fs.FS {
				fsys := memoryfs.New()
				fsys.MkdirAll(filepath.Join("b001", aspect.OrchestrionDirPathElement, "foo"), 0755)
				fsys.WriteFile(filepath.Join("b001", aspect.OrchestrionDirPathElement, "foo", "bar.go"), []byte("package foo\nfunc Bar() {}"), 0644)
				return fsys
			}(),
			want: []ModifiedFile{
				{
					modified: filepath.Join("b001", aspect.OrchestrionDirPathElement, "foo", "bar.go"),
				},
			},
		},
		{
			name: "simple",
			args: func() fs.FS {
				fsys := memoryfs.New()
				fsys.MkdirAll(filepath.Join("b001", aspect.OrchestrionDirPathElement, "foo"), 0755)
				fsys.WriteFile(filepath.Join("b001", aspect.OrchestrionDirPathElement, "foo", "bar.go"), []byte("//line foo/bar.go\npackage foo\nfunc Bar() {}"), 0644)
				return fsys
			}(),
			want: []ModifiedFile{
				{
					modified: filepath.Join("b001", aspect.OrchestrionDirPathElement, "foo", "bar.go"),
					original: filepath.Join("foo", "bar.go"),
				},
			},
		},
		{
			name: "sample",
			args: func() fs.FS {
				fsys := memoryfs.New()
				fsys.MkdirAll(filepath.Join("b001", aspect.OrchestrionDirPathElement, "github.com", "DataDog", "test"), 0755)
				fsys.WriteFile(filepath.Join("b001", aspect.OrchestrionDirPathElement, "github.com", "DataDog", "test", "bar.go"), []byte("//line /home/test/go/pkg/mod/github.com/!data!dog/test/bar.go\n"), 0644)
				return fsys
			}(),
			want: []ModifiedFile{
				{
					modified: filepath.Join("b001", aspect.OrchestrionDirPathElement, "github.com", "DataDog", "test", "bar.go"),
					original: filepath.Join("/home", "test", "go", "pkg", "mod", "github.com", "!data!dog", "test", "bar.go"),
				},
			},
		},
		{
			name: "multiple-files",
			args: func() fs.FS {
				fsys := memoryfs.New()
				fsys.MkdirAll(filepath.Join("b001", aspect.OrchestrionDirPathElement, "pkg"), 0755)
				fsys.WriteFile(filepath.Join("b001", aspect.OrchestrionDirPathElement, "pkg", "file1.go"), []byte("//line pkg/file1.go\npackage pkg\nfunc File1() {}"), 0644)
				fsys.WriteFile(filepath.Join("b001", aspect.OrchestrionDirPathElement, "pkg", "file2.go"), []byte("//line pkg/file2.go\npackage pkg\nfunc File2() {}"), 0644)
				return fsys
			}(),
			want: []ModifiedFile{
				{
					modified: filepath.Join("b001", aspect.OrchestrionDirPathElement, "pkg", "file1.go"),
					original: filepath.Join("pkg", "file1.go"),
				},
				{
					modified: filepath.Join("b001", aspect.OrchestrionDirPathElement, "pkg", "file2.go"),
					original: filepath.Join("pkg", "file2.go"),
				},
			},
		},
		{
			name: "missing-original-path",
			args: func() fs.FS {
				fsys := memoryfs.New()
				fsys.MkdirAll(filepath.Join("b001", aspect.OrchestrionDirPathElement, "pkg"), 0755)
				fsys.WriteFile(filepath.Join("b001", aspect.OrchestrionDirPathElement, "pkg", "file.go"), []byte("package pkg\nfunc File() {}"), 0644)
				return fsys
			}(),
			want: []ModifiedFile{
				{
					modified: filepath.Join("b001", aspect.OrchestrionDirPathElement, "pkg", "file.go"),
				},
			},
		},
		{
			name: "nested-directories",
			args: func() fs.FS {
				fsys := memoryfs.New()
				fsys.MkdirAll(filepath.Join("b001", aspect.OrchestrionDirPathElement, "pkg", "subpkg"), 0755)
				fsys.WriteFile(filepath.Join("b001", aspect.OrchestrionDirPathElement, "pkg", "subpkg", "file.go"), []byte("//line pkg/subpkg/file.go\npackage subpkg\nfunc File() {}"), 0644)
				return fsys
			}(),
			want: []ModifiedFile{
				{
					modified: filepath.Join("b001", aspect.OrchestrionDirPathElement, "pkg", "subpkg", "file.go"),
					original: filepath.Join("pkg", "subpkg", "file.go"),
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := fromWorkFS(context.Background(), "/tmp/build", tt.args)
			if (err != nil) != tt.wantErr {
				t.Errorf("fromWorkFS() error = %#v, wantErr %#v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got.files, tt.want) {
				t.Errorf("fromWorkFS() got = %#v, want %#v", got.files, tt.want)
			}
		})
	}
}
