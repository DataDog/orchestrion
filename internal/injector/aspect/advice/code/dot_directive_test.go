// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2025-present Datadog, Inc.

package code

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_parseDirectiveArgs(t *testing.T) {
	testCases := []struct {
		name    string
		prefix  string
		comment string
		want    []DirectiveArgument
		wantOk  bool
	}{
		{
			name:    "valid directive with two args",
			prefix:  "//dd:span",
			comment: "//dd:span span.name:rootHandler resource.name:\"GET /\"",
			want: []DirectiveArgument{
				{Key: "span.name", Value: "rootHandler"},
				{Key: "resource.name", Value: "GET /"},
			},
			wantOk: true,
		},
		{
			name:    "args with spaces double quote",
			prefix:  "//dd:span",
			comment: "//dd:span span.name:rootHandler resource.name:\"GET /\" foo:\"bar\" ",
			want: []DirectiveArgument{
				{Key: "span.name", Value: "rootHandler"},
				{Key: "resource.name", Value: "GET /"},
				{Key: "foo", Value: "bar"},
			},
			wantOk: true,
		},
		{
			name:    "args with spaces single quote",
			prefix:  "//dd:span",
			comment: "//dd:span span.name:rootHandler resource.name:'GET /' foo:'bar'",
			want: []DirectiveArgument{
				{Key: "span.name", Value: "rootHandler"},
				{Key: "resource.name", Value: "GET /"},
				{Key: "foo", Value: "bar"},
			},
			wantOk: true,
		},
		{
			name:    "single and double quotes",
			prefix:  "//dd:span",
			comment: `//dd:span span.name:'root handler' resource.name:"GET /home" "key with spaces":'value with spaces'`,
			want: []DirectiveArgument{
				{Key: "span.name", Value: "root handler"},
				{Key: "resource.name", Value: "GET /home"},
				{Key: "key with spaces", Value: "value with spaces"},
			},
			wantOk: true,
		},
		{
			name:    "valid directive with one arg",
			prefix:  "//dd:span",
			comment: "//dd:span service.name:my-service",
			want: []DirectiveArgument{
				{Key: "service.name", Value: "my-service"},
			},
			wantOk: true,
		},
		{
			name:    "prefix matches at start but is not full word",
			prefix:  "//dd:span",
			comment: "//dd:spanExtra service.name:my-service",
			want:    nil,
			wantOk:  false,
		},
		{
			name:    "non-matching prefix",
			prefix:  "//dd:span",
			comment: "//other:span span.name:foo",
			want:    nil,
			wantOk:  false,
		},
		{
			name:    "only prefix with no arguments",
			prefix:  "//dd:span",
			comment: "//dd:span",
			want:    []DirectiveArgument{},
			wantOk:  true,
		},
		{
			name:    "arg with only key and no colon",
			prefix:  "//dd:span",
			comment: "//dd:span standalone_arg",
			want: []DirectiveArgument{
				{Key: "standalone_arg", Value: ""},
			},
			wantOk: true,
		},
		{
			name:    "arg with multiple colons",
			prefix:  "//dd:span",
			comment: "//dd:span foo:bar:baz",
			want: []DirectiveArgument{
				{Key: "foo", Value: "bar:baz"},
			},
			wantOk: true,
		},
	}
	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := parseDirectiveArgs(tt.prefix, tt.comment)
			assert.Equal(t, tt.wantOk, ok)
			assert.Equal(t, tt.want, got)
		})
	}
}
