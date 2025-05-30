// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2025 Datadog, Inc.

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
					ModifiedPath: filepath.Join("b001", aspect.OrchestrionDirPathElement, "foo", "bar.go"),
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
					ModifiedPath: filepath.Join("b001", aspect.OrchestrionDirPathElement, "foo", "bar.go"),
					OriginalPath: filepath.Join("foo", "bar.go"),
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
					ModifiedPath: filepath.Join("b001", aspect.OrchestrionDirPathElement, "github.com", "DataDog", "test", "bar.go"),
					OriginalPath: "/home/test/go/pkg/mod/github.com/!data!dog/test/bar.go",
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
			if !reflect.DeepEqual(got.Files, tt.want) {
				t.Errorf("fromWorkFS() got = %#v, want %#v", got, tt.want)
			}
		})
	}
}
