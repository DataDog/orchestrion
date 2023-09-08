package instrument

import (
	"fmt"
	"io"
	"strings"
	"testing"

	"github.com/datadog/orchestrion/internal/config"

	"github.com/stretchr/testify/require"
)

func TestGin(t *testing.T) {
	var codeTpl = `package main

import "github.com/gin-gonic/gin"

func register() {
	%s
}
`
	var wantTpl = `package main

import (
	"github.com/datadog/orchestrion/instrument"
	"github.com/gin-gonic/gin"
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
		in   string
		want string
		tmpl string
	}{
		{in: `g := gin.New()`, want: `g.Use(instrument.GinMiddleware())`, tmpl: wantTpl},
		{in: `g := gin.Default()`, want: `g.Use(instrument.GinMiddleware())`, tmpl: wantTpl},
	}

	for i, tc := range tests {
		t.Run(fmt.Sprintf("tc-%d", i), func(t *testing.T) {
			code := fmt.Sprintf(codeTpl, tc.in)
			reader, err := InstrumentFile("test", strings.NewReader(code), config.Config{})
			require.Nil(t, err)
			got, err := io.ReadAll(reader)
			require.Nil(t, err)
			want := fmt.Sprintf(tc.tmpl, tc.in, tc.want)
			require.Equal(t, want, string(got))

			reader, err = UninstrumentFile("test", strings.NewReader(want), config.Config{})
			require.Nil(t, err)
			orig, err := io.ReadAll(reader)
			require.Nil(t, err)
			require.Equal(t, code, string(orig))
		})
	}
}

func TestGinDuplicates(t *testing.T) {
	var tpl = `package main

import (
	"net/http"

	"github.com/datadog/orchestrion/instrument"
	"github.com/gin-gonic/gin"
)

func ginServer() {
	//dd:instrumented
	r := gin.Default()
	//dd:startinstrument
	r.Use(instrument.GinMiddleware())
	//dd:endinstrument
	r.GET("/ping", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"message": "pong",
		})
	})
	r.Run()
}
`

	reader, err := InstrumentFile("test", strings.NewReader(tpl), config.Config{})
	require.Nil(t, err)
	got, err := io.ReadAll(reader)
	require.Nil(t, err)
	require.Equal(t, tpl, string(got))
}
