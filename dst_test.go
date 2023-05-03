package orchestrion

import (
	"fmt"
	"io"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestWrapHandler(t *testing.T) {
	var codeTpl = `package main

import (
	"net/http"
)

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
		{in: `s.Handle("/handle", handler)`, want: `s.Handle("/handle", orchestrion.WrapHandler(handler))`},
		{in: `s.Handle("/handle", http.HandlerFunc(myHandler))`, want: `s.Handle("/handle", orchestrion.WrapHandler(http.HandlerFunc(myHandler)))`},
		{in: `s.Handle("/handle",http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))`, want: `s.Handle("/handle", orchestrion.WrapHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})))`},
		{in: `s.HandleFunc("/handle", handler)`, want: `s.HandleFunc("/handle", orchestrion.WrapHandlerFunc(handler))`},
		{in: `s.HandleFunc("/handle", http.HandlerFunc(myHandler))`, want: `s.HandleFunc("/handle", orchestrion.WrapHandlerFunc(http.HandlerFunc(myHandler)))`},
		{in: `s.HandleFunc("/handle", func(w http.ResponseWriter, r *http.Request) {})`, want: `s.HandleFunc("/handle", orchestrion.WrapHandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))`},
	}

	for _, tc := range tests {
		code := fmt.Sprintf(codeTpl, tc.in)
		reader, err := ScanFile("test", strings.NewReader(code))
		require.Nil(t, err)
		got, err := io.ReadAll(reader)
		require.Nil(t, err)
		require.Equal(t, string(got), fmt.Sprintf(wantTpl, tc.want))
	}
}
