// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

// Copyright 2020 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package quoted

import (
	"reflect"
	"strings"
	"testing"
)

func TestSplit(t *testing.T) {
	for _, test := range []struct {
		name    string
		value   string
		want    []string
		wantErr string
	}{
		{name: "empty", value: "", want: nil},
		{name: "space", value: " ", want: nil},
		{name: "one", value: "a", want: []string{"a"}},
		{name: "leading_space", value: " a", want: []string{"a"}},
		{name: "trailing_space", value: "a ", want: []string{"a"}},
		{name: "two", value: "a b", want: []string{"a", "b"}},
		{name: "two_multi_space", value: "a  b", want: []string{"a", "b"}},
		{name: "two_tab", value: "a\tb", want: []string{"a", "b"}},
		{name: "two_newline", value: "a\nb", want: []string{"a", "b"}},
		{name: "quote_single", value: `'a b'`, want: []string{"a b"}},
		{name: "quote_double", value: `"a b"`, want: []string{"a b"}},
		{name: "quote_both", value: `'a '"b "`, want: []string{"a ", "b "}},
		{name: "quote_contains", value: `'a "'"'b"`, want: []string{`a "`, `'b`}},
		{name: "escape", value: `\'`, want: []string{`\'`}},
		{name: "quote_unclosed", value: `'a`, wantErr: "unterminated ' string"},
	} {
		t.Run(test.name, func(t *testing.T) {
			got, err := Split(test.value)
			if err != nil {
				if test.wantErr == "" {
					t.Fatalf("unexpected error: %v", err)
				} else if errMsg := err.Error(); !strings.Contains(errMsg, test.wantErr) {
					t.Fatalf("error %q does not contain %q", errMsg, test.wantErr)
				}
				return
			}
			if test.wantErr != "" {
				t.Fatalf("unexpected success; wanted error containing %q", test.wantErr)
			}
			if !reflect.DeepEqual(got, test.want) {
				t.Errorf("got %q; want %q", got, test.want)
			}
		})
	}
}
