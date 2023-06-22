// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package instrument

import (
	"fmt"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/datadog/orchestrion/internal/config"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMultipleWrap(t *testing.T) {
	var codeTpl = `package main

import "net/http"

var s http.ServeMux

func register() {
	%s
}
`

	var wantTpl = `package main

import (
	"net/http"

	"github.com/datadog/orchestrion/instrument"
)

var s http.ServeMux

func register() {
	%s
}
`
	tests := []struct {
		in   string
		want string
	}{
		{
			in: `http.Handle("/handle", handler)
	http.Handle("/other", handler2)`,
			want: `//dd:startwrap
	http.Handle("/handle", instrument.WrapHandler(handler))
	//dd:endwrap
	//dd:startwrap
	http.Handle("/other", instrument.WrapHandler(handler2))
	//dd:endwrap`,
		},
	}

	for _, tc := range tests {
		t.Run("", func(t *testing.T) {
			code := fmt.Sprintf(codeTpl, tc.in)
			reader, err := InstrumentFile("test", strings.NewReader(code), config.Default)
			require.Nil(t, err)
			got, err := io.ReadAll(reader)
			require.Nil(t, err)
			want := fmt.Sprintf(wantTpl, tc.want)
			require.Equal(t, want, string(got))

			reader, err = UninstrumentFile("test", strings.NewReader(want), config.Default)
			require.Nil(t, err)
			orig, err := io.ReadAll(reader)
			require.Nil(t, err)
			require.Equal(t, code, string(orig))
		})
	}

}

func TestWrapHandlerExpr(t *testing.T) {
	var codeTpl = `package main

import "net/http"

var s http.ServeMux

func register() {
	%s
}
`
	var wantTpl = `package main

import (
	"net/http"

	"github.com/datadog/orchestrion/instrument"
)

var s http.ServeMux

func register() {
	//dd:startwrap
	%s
	//dd:endwrap
}
`
	tests := []struct {
		in   string
		want string
	}{
		{in: `http.Handle("/handle", handler)`, want: `http.Handle("/handle", instrument.WrapHandler(handler))`},
		{in: `http.Handle("/handle", http.HandlerFunc(myHandler))`, want: `http.Handle("/handle", instrument.WrapHandler(http.HandlerFunc(myHandler)))`},
		{in: `http.Handle("/handle", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))`, want: `http.Handle("/handle", instrument.WrapHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})))`},
		{in: `http.HandleFunc("/handle", handler)`, want: `http.HandleFunc("/handle", instrument.WrapHandlerFunc(handler))`},
		{in: `http.HandleFunc("/handle", http.HandlerFunc(myHandler))`, want: `http.HandleFunc("/handle", instrument.WrapHandlerFunc(http.HandlerFunc(myHandler)))`},
		{in: `http.HandleFunc("/handle", func(w http.ResponseWriter, r *http.Request) {})`, want: `http.HandleFunc("/handle", instrument.WrapHandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))`},
		{in: `s.Handle("/handle", handler)`, want: `s.Handle("/handle", instrument.WrapHandler(handler))`},
		{in: `s.Handle("/handle", http.HandlerFunc(myHandler))`, want: `s.Handle("/handle", instrument.WrapHandler(http.HandlerFunc(myHandler)))`},
		{in: `s.Handle("/handle", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))`, want: `s.Handle("/handle", instrument.WrapHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})))`},
		{in: `s.HandleFunc("/handle", handler)`, want: `s.HandleFunc("/handle", instrument.WrapHandlerFunc(handler))`},
		{in: `s.HandleFunc("/handle", http.HandlerFunc(myHandler))`, want: `s.HandleFunc("/handle", instrument.WrapHandlerFunc(http.HandlerFunc(myHandler)))`},
		{in: `s.HandleFunc("/handle", func(w http.ResponseWriter, r *http.Request) {})`, want: `s.HandleFunc("/handle", instrument.WrapHandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))`},
	}

	for _, tc := range tests {
		t.Run("", func(t *testing.T) {
			code := fmt.Sprintf(codeTpl, tc.in)
			reader, err := InstrumentFile("test", strings.NewReader(code), config.Default)
			require.Nil(t, err)
			got, err := io.ReadAll(reader)
			require.Nil(t, err)
			want := fmt.Sprintf(wantTpl, tc.want)
			require.Equal(t, want, string(got))

			reader, err = UninstrumentFile("test", strings.NewReader(want), config.Default)
			require.Nil(t, err)
			orig, err := io.ReadAll(reader)
			require.Nil(t, err)
			require.Equal(t, code, string(orig))
		})
	}
}

func TestWrapHandlerAssign(t *testing.T) {
	var codeTpl = `package main

import "net/http"

var s *http.Server

func register() {
	s = &http.Server{
		Addr:    ":8080",
		Handler: %s,
	}
}
`
	var wantTpl = `package main

import (
	"net/http"

	"github.com/datadog/orchestrion/instrument"
)

var s *http.Server

func register() {
	s = &http.Server{
		Addr: ":8080",
		//dd:startwrap
		Handler: %s,
		//dd:endwrap
	}
}
`
	tests := []struct {
		in   string
		want string
	}{
		{in: `http.HandlerFunc(myHandler)`, want: `instrument.WrapHandler(http.HandlerFunc(myHandler))`},
		{in: `myHandler`, want: `instrument.WrapHandler(myHandler)`},
		{in: `NewHandler()`, want: `instrument.WrapHandler(NewHandler())`},
		{in: `&handler{}`, want: `instrument.WrapHandler(&handler{})`},
	}

	for _, tc := range tests {
		t.Run("", func(t *testing.T) {
			code := fmt.Sprintf(codeTpl, tc.in)
			reader, err := InstrumentFile("test", strings.NewReader(code), config.Default)
			require.Nil(t, err)
			got, err := io.ReadAll(reader)
			require.Nil(t, err)
			want := fmt.Sprintf(wantTpl, tc.want)
			require.Equal(t, want, string(got))

			// TODO: Implement unwrapHandlerAssign to uncomment the following assertions!
			//
			// reader, err = UninstrumentFile("test", strings.NewReader(want))
			// require.Nil(t, err)
			// orig, err := io.ReadAll(reader)
			// require.Nil(t, err)
			// require.Equal(t, code, string(orig))
		})
	}
}

func TestWrapClientAssign(t *testing.T) {
	var codeTpl = `package main

import "net/http"

var c *http.Client

func init() {
	c = %s
}
`
	var wantTpl = `package main

import (
	"net/http"

	"github.com/datadog/orchestrion/instrument"
)

var c *http.Client

func init() {
	//dd:startwrap
	c = %s
	//dd:endwrap
}
`
	tests := []struct {
		in   string
		want string
	}{
		{in: `&http.Client{Timeout: time.Second}`, want: `instrument.WrapHTTPClient(&http.Client{Timeout: time.Second})`},
		{in: `MyClient()`, want: `instrument.WrapHTTPClient(MyClient())`},
		{in: `http.DefaultClient`, want: `instrument.WrapHTTPClient(http.DefaultClient)`},
	}

	for _, tc := range tests {
		t.Run("", func(t *testing.T) {
			code := fmt.Sprintf(codeTpl, tc.in)
			reader, err := InstrumentFile("test", strings.NewReader(code), config.Default)
			require.Nil(t, err)
			got, err := io.ReadAll(reader)
			require.Nil(t, err)
			want := fmt.Sprintf(wantTpl, tc.want)
			require.Equal(t, want, string(got))

			reader, err = UninstrumentFile("test", strings.NewReader(want), config.Default)
			require.Nil(t, err)
			orig, err := io.ReadAll(reader)
			require.Nil(t, err)
			require.Equal(t, code, string(orig))
		})
	}
}

func TestSpanInstrumentation(t *testing.T) {
	codef := func(s string) string {
		var code = `package main

import "context"

//dd:span foo:bar other:tag
func MyFunc(somectx context.Context) {%s}
`

		return fmt.Sprintf(code, s)
	}

	wantf := func(s string) string {
		var want = `package main

import (
	"context"

	"github.com/datadog/orchestrion/instrument"
	"github.com/datadog/orchestrion/instrument/event"
)

//dd:span foo:bar other:tag
func MyFunc(somectx context.Context) {
	//dd:startinstrument
	somectx = instrument.Report(somectx, event.EventStart, "function-name", "MyFunc", "foo", "bar", "other", "tag")
	defer instrument.Report(somectx, event.EventEnd, "function-name", "MyFunc", "foo", "bar", "other", "tag")
	//dd:endinstrument%s
}
`
		return fmt.Sprintf(want, s)
	}

	for _, tt := range []struct {
		in, out string
	}{
		{in: "", out: ""},
		{in: "\n\twhatever.Code()\n", out: "\n\twhatever.Code()"},
	} {
		t.Run("", func(t *testing.T) {
			var code = codef(tt.in)
			var want = wantf(tt.out)
			reader, err := InstrumentFile("test", strings.NewReader(code), config.Default)
			require.NoError(t, err)
			got, err := io.ReadAll(reader)
			require.NoError(t, err)
			require.Equal(t, want, string(got))

			reader, err = UninstrumentFile("test", strings.NewReader(want), config.Default)
			require.Nil(t, err)
			orig, err := io.ReadAll(reader)
			require.Nil(t, err)
			require.Equal(t, code, string(orig))
		})
	}
}

func TestMainInstrumentation(t *testing.T) {
	var code = `package main

func main() {
	whatever.code
}
`
	var want = `package main

import "github.com/datadog/orchestrion/instrument"

func main() {
	//dd:startinstrument
	defer instrument.Init()()
	//dd:endinstrument
	whatever.code
}
`

	reader, err := InstrumentFile("test", strings.NewReader(code), config.Default)
	require.NoError(t, err)
	got, err := io.ReadAll(reader)
	require.NoError(t, err)
	assert.Equal(t, want, string(got))

	reader, err = UninstrumentFile("test", strings.NewReader(want), config.Default)
	require.Nil(t, err)
	orig, err := io.ReadAll(reader)
	require.Nil(t, err)
	require.Equal(t, code, string(orig))
}

func TestHTTPModeConfig(t *testing.T) {
	for _, tc := range []struct {
		in, out, mode string
	}{
		{in: "./testdata/http_in.go", out: "./testdata/http_wrapped.go", mode: "wrap"},
		{in: "./testdata/http_in.go", out: "./testdata/http_reported.go", mode: "report"},
	} {
		t.Run(tc.mode, func(t *testing.T) {
			in, err := os.Open(tc.in)
			require.NoError(t, err)

			reader, err := InstrumentFile(in.Name(), in, config.Config{HTTPMode: tc.mode})
			require.NoError(t, err)

			got, err := io.ReadAll(reader)
			require.NoError(t, err)

			want, err := os.ReadFile(tc.out)
			require.NoError(t, err)

			require.Equal(t, string(want), string(got))
		})
	}
}

func TestWrapSqlExpr(t *testing.T) {
	var codeTpl = `package main

import "database/sql"

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

	var wantTpl2 = `package main

import (
	"database/sql"

	"github.com/datadog/orchestrion/instrument"
)

func register() {
	%s
}
`

	tests := []struct {
		in   string
		want string
		tmpl string
	}{
		{in: `db, err := sql.Open("db", "mypath")`, want: `db, err := instrument.Open("db", "mypath")`, tmpl: wantTpl},
		{in: `db := sql.OpenDB(connector)`, want: `db := instrument.OpenDB(connector)`, tmpl: wantTpl},
		{in: `return sql.Open("db", "mypath")`, want: `return instrument.Open("db", "mypath")`, tmpl: wantTpl},
		{in: `return sql.OpenDB(connector)`, want: `return instrument.OpenDB(connector)`, tmpl: wantTpl},

		{
			in: `func() (*sql.DB, error) {
		return sql.Open("db", "mypath")
	}()`,
			want: `func() (*sql.DB, error) {
		//dd:startwrap
		return instrument.Open("db", "mypath")
		//dd:endwrap
	}()`,
			tmpl: wantTpl2,
		},

		{
			in: `f := func() (*sql.DB, error) {
		return sql.Open("db", "mypath")
	}`,
			want: `f := func() (*sql.DB, error) {
		//dd:startwrap
		return instrument.Open("db", "mypath")
		//dd:endwrap
	}`,
			tmpl: wantTpl2,
		},
	}

	for _, tc := range tests {
		t.Run("", func(t *testing.T) {
			code := fmt.Sprintf(codeTpl, tc.in)
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

func TestWrapGRPCServer(t *testing.T) {
	var codeTpl = `package main

import "google.golang.org/grpc"

var s *grpc.Server

func init() {
	s = %s
}
`
	var wantTpl = `package main

import (
	"github.com/datadog/orchestrion/instrument"
	"google.golang.org/grpc"
)

var s *grpc.Server

func init() {
	//dd:startwrap
	s = %s
	//dd:endwrap
}
`
	tests := []struct {
		in   string
		want string
	}{
		{in: `grpc.NewServer()`, want: `grpc.NewServer(instrument.GRPCStreamServerInterceptor(), instrument.GRPCUnaryServerInterceptor())`},
		{in: `grpc.NewServer(opt1, opt2)`, want: `grpc.NewServer(opt1, opt2, instrument.GRPCStreamServerInterceptor(), instrument.GRPCUnaryServerInterceptor())`},
	}

	for _, tc := range tests {
		t.Run("", func(t *testing.T) {
			code := fmt.Sprintf(codeTpl, tc.in)
			reader, err := InstrumentFile("test", strings.NewReader(code), config.Default)
			require.Nil(t, err)
			got, err := io.ReadAll(reader)
			require.Nil(t, err)
			want := fmt.Sprintf(wantTpl, tc.want)
			require.Equal(t, want, string(got))

			reader, err = UninstrumentFile("test", strings.NewReader(want), config.Default)
			require.Nil(t, err)
			orig, err := io.ReadAll(reader)
			require.Nil(t, err)
			require.Equal(t, code, string(orig))
		})
	}
}

func TestWrapGRPCClient(t *testing.T) {
	var codeTpl = `package main

import "google.golang.org/grpc"

var c *grpc.ClientConn

func init() {
	var err error
	c, err = %s
}
`
	var wantTpl = `package main

import (
	"github.com/datadog/orchestrion/instrument"
	"google.golang.org/grpc"
)

var c *grpc.ClientConn

func init() {
	var err error
	//dd:startwrap
	c, err = %s
	//dd:endwrap
}
`
	tests := []struct {
		in   string
		want string
	}{
		{in: `grpc.Dial("localhost:8888")`, want: `grpc.Dial("localhost:8888", instrument.GRPCStreamClientInterceptor(), instrument.GRPCUnaryClientInterceptor())`},
		{in: `grpc.Dial("localhost:8888", opt1, opt2)`, want: `grpc.Dial("localhost:8888", opt1, opt2, instrument.GRPCStreamClientInterceptor(), instrument.GRPCUnaryClientInterceptor())`},
	}

	for _, tc := range tests {
		t.Run("", func(t *testing.T) {
			code := fmt.Sprintf(codeTpl, tc.in)
			reader, err := InstrumentFile("test", strings.NewReader(code), config.Default)
			require.Nil(t, err)
			got, err := io.ReadAll(reader)
			require.Nil(t, err)
			want := fmt.Sprintf(wantTpl, tc.want)
			require.Equal(t, want, string(got))

			reader, err = UninstrumentFile("test", strings.NewReader(want), config.Default)
			require.Nil(t, err)
			orig, err := io.ReadAll(reader)
			require.Nil(t, err)
			require.Equal(t, code, string(orig))
		})
	}
}

func TestIgnore(t *testing.T) {
	var grpcClientTmpl = `package main

import "google.golang.org/grpc"

var c *grpc.ClientConn

func init() {
	var err error
	//dd:ignore
	c, err = %s
}
`

	var grpcServerTmpl = `package main

import "google.golang.org/grpc"

var s *grpc.Server

func init() {
	//dd:ignore
	s = %s
}
`

	var sqlTmpl = `package main

import "database/sql"

func register() {
	//dd:ignore
	%s
}
`

	var ddspanTmpl = `package main

import "context"

//dd:ignore
//dd:span foo:bar other:tag
func MyFunc(somectx context.Context) {
	fmt.Printf("%s")
}
`

	var handleTmpl = `package main

import "net/http"

var s http.ServeMux

func register() {
	//dd:ignore
	%s
}
`

	tests := []struct {
		in   string
		tmpl string
	}{
		{in: `grpc.Dial("localhost:8888")`, tmpl: grpcClientTmpl},
		{in: `grpc.Dial("localhost:8888", opt1, opt2)`, tmpl: grpcClientTmpl},
		{in: `grpc.NewServer()`, tmpl: grpcServerTmpl},
		{in: `grpc.NewServer(opt1, opt2)`, tmpl: grpcServerTmpl},

		{in: `db, err := sql.Open("db", "mypath")`, tmpl: sqlTmpl},
		{in: `db := sql.OpenDB(connector)`, tmpl: sqlTmpl},
		{in: `return sql.Open("db", "mypath")`, tmpl: sqlTmpl},
		{in: `return sql.OpenDB(connector)`, tmpl: sqlTmpl},

		{
			in: `func() (*sql.DB, error) {
		return sql.Open("db", "mypath")
	}()`,
			tmpl: sqlTmpl,
		},

		{
			in: `f := func() (*sql.DB, error) {
		return sql.Open("db", "mypath")
	}`,
			tmpl: sqlTmpl,
		},

		{in: `doesn't matter.\n`, tmpl: ddspanTmpl},

		{in: `http.Handle("/handle", handler)`, tmpl: handleTmpl},
		{in: `http.Handle("/handle", http.HandlerFunc(myHandler))`, tmpl: handleTmpl},
		{in: `http.Handle("/handle", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))`, tmpl: handleTmpl},
		{in: `http.HandleFunc("/handle", handler)`, tmpl: handleTmpl},
		{in: `http.HandleFunc("/handle", http.HandlerFunc(myHandler))`, tmpl: handleTmpl},
		{in: `http.HandleFunc("/handle", func(w http.ResponseWriter, r *http.Request) {})`, tmpl: handleTmpl},
		{in: `s.Handle("/handle", handler)`, tmpl: handleTmpl},
		{in: `s.Handle("/handle", http.HandlerFunc(myHandler))`, tmpl: handleTmpl},
		{in: `s.Handle("/handle", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))`, tmpl: handleTmpl},
		{in: `s.HandleFunc("/handle", handler)`, tmpl: handleTmpl},
		{in: `s.HandleFunc("/handle", http.HandlerFunc(myHandler))`, tmpl: handleTmpl},
		{in: `s.HandleFunc("/handle", func(w http.ResponseWriter, r *http.Request) {})`, tmpl: handleTmpl},
	}

	for _, tc := range tests {
		t.Run("", func(t *testing.T) {
			code := fmt.Sprintf(tc.tmpl, tc.in)
			reader, err := InstrumentFile("test", strings.NewReader(code), config.Default)
			require.Nil(t, err)
			got, err := io.ReadAll(reader)
			require.Nil(t, err)
			require.Equal(t, code, string(got))
		})
	}

	t.Run("func", func(t *testing.T) {
		var code = `package main

import (
	"database/sql"
	"net/http"

	"google.golang.org/grpc"
)

var c *grpc.ClientConn

//dd:ignore
func init() {
	var err error
	c, err = grpc.Dial("localhost:8888")

	http.Handle("/handle", handler)

	db, err := sql.Open("db", "mypath")
}
`
		reader, err := InstrumentFile("test", strings.NewReader(code), config.Default)
		require.Nil(t, err)
		got, err := io.ReadAll(reader)
		require.Nil(t, err)
		require.Equal(t, code, string(got))

	})
}
