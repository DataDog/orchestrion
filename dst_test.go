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

import (
	"net/http"
)

func register() {
	%s
}
`
	var wantTpl = `package main

import (
	"net/http"

	"github.com/datadog/orchestrion"
)

func register() {
	//dd:startinstrument
	%s
	//dd:endinstrument
}
`
	tests := []struct {
		in   string
		want string
	}{
		{in: `http.Handle("/handle", handler)`, want: `http.Handle("/handle", orchestrion.WrapHandler(handler))`},
		{in: `http.Handle("/handle", http.HandlerFunc(myHandler))`, want: `http.Handle("/handle", orchestrion.WrapHandler(http.HandlerFunc(myHandler)))`},
		{in: `http.Handle("/handle",http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))`, want: `http.Handle("/handle", orchestrion.WrapHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})))`},
		{in: `http.HandleFunc("/handle", handler)`, want: `http.HandleFunc("/handle", orchestrion.WrapHandlerFunc(handler))`},
		{in: `http.HandleFunc("/handle", http.HandlerFunc(myHandler))`, want: `http.HandleFunc("/handle", orchestrion.WrapHandlerFunc(http.HandlerFunc(myHandler)))`},
		{in: `http.HandleFunc("/handle", func(w http.ResponseWriter, r *http.Request) {})`, want: `http.HandleFunc("/handle", orchestrion.WrapHandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))`},
	}

	for _, tc := range tests {
		code := fmt.Sprintf(codeTpl, tc.in)
		reader, err := ScanFile("test", strings.NewReader(code))
		require.Nil(t, err)
		got, err := io.ReadAll(reader)
		require.Nil(t, err)
		require.Equal(t, fmt.Sprintf(wantTpl, tc.want), string(got))
	}
}

func TestSpanInstrumentation(t *testing.T) {
	var code = `package main

import (
	"context"
)

//dd:span foo:bar other:tag
func MyFunc(somectx context.Context) {
	whatever.code
}
`
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
	//dd:endinstrument
	whatever.code
}
`

	reader, err := ScanFile("test", strings.NewReader(code))
	require.NoError(t, err)
	got, err := io.ReadAll(reader)
	require.NoError(t, err)
	assert.Equal(t, want, string(got))
}
