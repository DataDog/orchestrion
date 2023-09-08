package instrument

import (
	"fmt"
	"io"
	"strings"
	"testing"

	"github.com/datadog/orchestrion/internal/config"

	"github.com/stretchr/testify/require"
)

func TestEchoV4(t *testing.T) {
	var codeTpl = `package main

import %s

func register() {
	%s
}
`
	var wantTpl = `package main

import (
	"github.com/datadog/orchestrion/instrument"
	%s
)

func register() {
	//dd:instrumented
	%s
	//dd:startinstrument
	%s
	//dd:endinstrument
}
`

	tests := []struct {
		pkg  string
		stmt string
		want string
		tmpl string
	}{
		{pkg: `"github.com/labstack/echo/v4"`, stmt: `r := echo.New()`, want: `r.Use(instrument.EchoV4Middleware())`, tmpl: wantTpl},
		{pkg: `echo "github.com/labstack/echo/v4"`, stmt: `r := echo.New()`, want: `r.Use(instrument.EchoV4Middleware())`, tmpl: wantTpl},
		{pkg: `echov4 "github.com/labstack/echo/v4"`, stmt: `r := echov4.New()`, want: `r.Use(instrument.EchoV4Middleware())`, tmpl: wantTpl},
	}

	for i, tc := range tests {
		t.Run(fmt.Sprintf("tc-%d", i), func(t *testing.T) {
			code := fmt.Sprintf(codeTpl, tc.pkg, tc.stmt)
			reader, err := InstrumentFile("test", strings.NewReader(code), config.Config{})
			require.Nil(t, err)
			got, err := io.ReadAll(reader)
			require.Nil(t, err)
			want := fmt.Sprintf(tc.tmpl, tc.pkg, tc.stmt, tc.want)
			require.Equal(t, want, string(got))

			reader, err = UninstrumentFile("test", strings.NewReader(want), config.Config{})
			require.Nil(t, err)
			orig, err := io.ReadAll(reader)
			require.Nil(t, err)
			require.Equal(t, code, string(orig))
		})
	}
}

func TestEchoV4Duplicates(t *testing.T) {
	var tpl = `package main

import (
	"github.com/datadog/orchestrion/instrument"
	"github.com/labstack/echo/v4"
)

func echoV4Server() {
	//dd:instrumented
	r := echo.New()
	//dd:startinstrument
	r.Use(instrument.EchoV4Middleware())
	//dd:endinstrument
}
`

	reader, err := InstrumentFile("test", strings.NewReader(tpl), config.Config{})
	require.Nil(t, err)
	got, err := io.ReadAll(reader)
	require.Nil(t, err)
	require.Equal(t, tpl, string(got))
}
