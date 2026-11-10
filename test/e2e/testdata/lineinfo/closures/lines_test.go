// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package closures

import (
	"bufio"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
)

var markRe = regexp.MustCompile(`// mark:(\S+)`)

func TestLines(t *testing.T) {
	want := map[string]struct {
		file string
		line int
	}{}
	f, err := os.Open("closures.go")
	if err != nil {
		t.Fatal(err)
	}
	sc := bufio.NewScanner(f)
	for n := 1; sc.Scan(); n++ {
		if m := markRe.FindStringSubmatch(sc.Text()); m != nil {
			want[m[1]] = struct {
				file string
				line int
			}{"closures/closures.go", n}
		}
	}
	f.Close()

	got := Bodies()

	names := make([]string, 0, len(got))
	for name := range got {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		if got[name].Line != want[name].line {
			t.Errorf("%-14s want line %2d, runtime says %2d", name, want[name].line, got[name].Line)
		}
		if !filepath.IsAbs(got[name].File) || !strings.HasSuffix(got[name].File, want[name].file) {
			t.Errorf("%-14s want absolute file ending in %q, runtime says %q", name, want[name].file, got[name].File)
		}
	}
}
