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

func TestGorillaMux(t *testing.T) {
	var codeTpl = `package main

import "github.com/gorilla/mux"

func register() {
	%s
}
`
	var wantTpl = `package main

import (
	"github.com/datadog/orchestrion/instrument"
	"github.com/gorilla/mux"
)

func register() {
	//dd:startwrap
	%s
	//dd:endwrap
}
`

	tests := []struct {
		in   string
		want string
		tmpl string
	}{
		{in: `r := mux.NewRouter()`, want: `r := instrument.WrapGorillaMuxRouter(mux.NewRouter())`, tmpl: wantTpl},
	}

	for i, tc := range tests {
		t.Run(fmt.Sprintf("tc-%d", i), func(t *testing.T) {
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
