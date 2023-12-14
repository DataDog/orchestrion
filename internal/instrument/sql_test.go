// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package instrument

import (
	"fmt"
	"io"
	"strings"
	"testing"

	"github.com/datadog/orchestrion/internal/config"

	"github.com/stretchr/testify/require"
)

func TestSql(t *testing.T) {
	var codeTpl = `package main

import %s

func register() {
	%s
}
`
	var wantTpl = `package main

import "github.com/datadog/orchestrion/instrument"

func register() {
	//dd:startwrap
	%s
	//dd:endwrap
}
`

	tests := []struct {
		pkg  string
		stmt string
		want string
		tmpl string
	}{
		{pkg: `"database/sql"`, stmt: `r, err := sql.Open("driver", "connString")`, want: `r, err := instrument.Open("driver", "connString")`, tmpl: wantTpl},
		{pkg: `"database/sql"`, stmt: `db := sql.OpenDB(mssql.NewConnectorConfig(msdsn.Config{}))`, want: `db := instrument.OpenDB(mssql.NewConnectorConfig(msdsn.Config{}))`, tmpl: wantTpl},
	}

	for i, tc := range tests {
		t.Run(fmt.Sprintf("tc-%d", i), func(t *testing.T) {
			code := fmt.Sprintf(codeTpl, tc.pkg, tc.stmt)
			reader, err := InstrumentFile("test", strings.NewReader(code), config.Config{})
			require.Nil(t, err)
			got, err := io.ReadAll(reader)
			require.Nil(t, err)
			want := fmt.Sprintf(tc.tmpl, tc.want)
			require.Equal(t, want, string(got))

			reader, err = UninstrumentFile("test", strings.NewReader(want), config.Config{})
			require.Nil(t, err)
			orig, err := io.ReadAll(reader)
			require.Nil(t, err)
			require.Equal(t, code, string(orig))
		})
	}
}

func TestSqlWithOptions(t *testing.T) {
	var codeTpl = `package main

import %s

func register() {
	//dd:options service:test-name tag:my:value
	%s
}
`
	var wantTpl = `package main

import "github.com/datadog/orchestrion/instrument"

func register() {
	//dd:options service:test-name tag:my:value
	//dd:startwrap
	%s
	//dd:endwrap
}
`

	tests := []struct {
		pkg  string
		stmt string
		want string
		tmpl string
	}{
		{pkg: `"database/sql"`, stmt: `r, err := sql.Open("driver", "connString")`, want: `r, err := instrument.Open("driver", "connString", instrument.SqlWithServiceName("test-name"), instrument.SqlWithCustomTag("my", "value"))`, tmpl: wantTpl},
	}

	for i, tc := range tests {
		t.Run(fmt.Sprintf("tc-%d", i), func(t *testing.T) {
			code := fmt.Sprintf(codeTpl, tc.pkg, tc.stmt)
			reader, err := InstrumentFile("test", strings.NewReader(code), config.Config{})
			require.Nil(t, err)
			got, err := io.ReadAll(reader)
			require.Nil(t, err)
			want := fmt.Sprintf(tc.tmpl, tc.want)
			require.Equal(t, want, string(got))

			reader, err = UninstrumentFile("test", strings.NewReader(want), config.Config{})
			require.Nil(t, err)
			orig, err := io.ReadAll(reader)
			require.Nil(t, err)
			require.Equal(t, code, string(orig))
		})
	}
}
