// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package cgoline

import (
	"bufio"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"

	"example.com/lineinfo/lib"
)

var markRe = regexp.MustCompile(`// mark:(\S+)`)

// mark is the source position a marker comment asserts: the file the marker was
// found in, relative to the module, and its line.
type mark struct {
	file string
	line int
}

// marks returns the source position of every "// mark:<name>" comment.
func marks(t *testing.T, files ...string) map[string]mark {
	t.Helper()
	want := map[string]mark{}
	for _, name := range files {
		f, err := os.Open(name)
		if err != nil {
			t.Fatal(err)
		}
		sc := bufio.NewScanner(f)
		for n := 1; sc.Scan(); n++ {
			if m := markRe.FindStringSubmatch(sc.Text()); m != nil {
				want[m[1]] = mark{file: "cgoline/" + name, line: n}
			}
		}
		f.Close()
	}
	return want
}

func TestLines(t *testing.T) {
	if Twice(2) != 4 {
		t.Fatal("cgo call failed")
	}
	want := marks(t, "cgo.go", "plain.go")
	got := map[string]lib.Pos{}
	CgoLines(got)
	MultiLine(got)
	got["oneliner-line"], got["oneliner-target"] = OneLiner()
	want["oneliner-line"], want["oneliner-target"] = want["oneliner"], want["oneliner"]
	delete(want, "oneliner")

	names := make([]string, 0, len(got))
	for name := range got {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		if got[name].Line != want[name].line {
			t.Errorf("%-16s want line %2d, runtime says %2d", name, want[name].line, got[name].Line)
		}
		if !filepath.IsAbs(got[name].File) || !strings.HasSuffix(got[name].File, want[name].file) {
			t.Errorf("%-16s want absolute file ending in %q, runtime says %q", name, want[name].file, got[name].File)
		}
	}
}
