package orchestrion

import (
	"fmt"
	"io"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWrapHandler(t *testing.T) {
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

	"github.com/datadog/orchestrion"
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
		{in: `http.Handle("/handle", handler)`, want: `http.Handle("/handle", orchestrion.WrapHandler(handler))`},
		{in: `http.Handle("/handle", http.HandlerFunc(myHandler))`, want: `http.Handle("/handle", orchestrion.WrapHandler(http.HandlerFunc(myHandler)))`},
		{in: `http.Handle("/handle", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))`, want: `http.Handle("/handle", orchestrion.WrapHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})))`},
		{in: `http.HandleFunc("/handle", handler)`, want: `http.HandleFunc("/handle", orchestrion.WrapHandlerFunc(handler))`},
		{in: `http.HandleFunc("/handle", http.HandlerFunc(myHandler))`, want: `http.HandleFunc("/handle", orchestrion.WrapHandlerFunc(http.HandlerFunc(myHandler)))`},
		{in: `http.HandleFunc("/handle", func(w http.ResponseWriter, r *http.Request) {})`, want: `http.HandleFunc("/handle", orchestrion.WrapHandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))`},
		{in: `s.Handle("/handle", handler)`, want: `s.Handle("/handle", orchestrion.WrapHandler(handler))`},
		{in: `s.Handle("/handle", http.HandlerFunc(myHandler))`, want: `s.Handle("/handle", orchestrion.WrapHandler(http.HandlerFunc(myHandler)))`},
		{in: `s.Handle("/handle", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))`, want: `s.Handle("/handle", orchestrion.WrapHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})))`},
		{in: `s.HandleFunc("/handle", handler)`, want: `s.HandleFunc("/handle", orchestrion.WrapHandlerFunc(handler))`},
		{in: `s.HandleFunc("/handle", http.HandlerFunc(myHandler))`, want: `s.HandleFunc("/handle", orchestrion.WrapHandlerFunc(http.HandlerFunc(myHandler)))`},
		{in: `s.HandleFunc("/handle", func(w http.ResponseWriter, r *http.Request) {})`, want: `s.HandleFunc("/handle", orchestrion.WrapHandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))`},
	}

	for _, tc := range tests {
		t.Run("", func(t *testing.T) {
			code := fmt.Sprintf(codeTpl, tc.in)
			reader, err := InstrumentFile("test", strings.NewReader(code))
			require.Nil(t, err)
			got, err := io.ReadAll(reader)
			require.Nil(t, err)
			want := fmt.Sprintf(wantTpl, tc.want)
			require.Equal(t, want, string(got))

			reader, err = UninstrumentFile("test", strings.NewReader(want))
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

	"github.com/datadog/orchestrion"
)

//dd:span foo:bar other:tag
func MyFunc(somectx context.Context) {
	//dd:startinstrument
	somectx = orchestrion.Report(somectx, orchestrion.EventStart, "function-name", "MyFunc", "foo", "bar", "other", "tag")
	defer orchestrion.Report(somectx, orchestrion.EventEnd, "function-name", "MyFunc", "foo", "bar", "other", "tag")
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
			reader, err := InstrumentFile("test", strings.NewReader(code))
			require.NoError(t, err)
			got, err := io.ReadAll(reader)
			require.NoError(t, err)
			require.Equal(t, want, string(got))

			reader, err = UninstrumentFile("test", strings.NewReader(want))
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

import "github.com/datadog/orchestrion"

func main() {
	//dd:startinstrument
	defer orchestrion.Init()()
	//dd:endinstrument
	whatever.code
}
`

	reader, err := InstrumentFile("test", strings.NewReader(code))
	require.NoError(t, err)
	got, err := io.ReadAll(reader)
	require.NoError(t, err)
	assert.Equal(t, want, string(got))

	reader, err = UninstrumentFile("test", strings.NewReader(want))
	require.Nil(t, err)
	orig, err := io.ReadAll(reader)
	require.Nil(t, err)
	require.Equal(t, code, string(orig))
}
