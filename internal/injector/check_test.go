// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package injector

import (
	"fmt"
	goparser "go/parser"
	"go/token"
	"io"
	"runtime"
	"strconv"
	"testing"

	"github.com/DataDog/orchestrion/internal/injector/parse"
	"github.com/stretchr/testify/require"
)

const goFile = `
package main

func main() {}
`

func TestNewerGoVersion(t *testing.T) {
	fset := token.NewFileSet()
	astFile, err := goparser.ParseFile(fset, "main.go", []byte(goFile), goparser.ParseComments)
	require.NoError(t, err)

	versionInt, err := strconv.Atoi(runtime.Version()[4:6])
	require.NoError(t, err)

	injector := &Injector{
		GoVersion: fmt.Sprintf("go1.%d", versionInt+1), // +1 to always make sure we have a newer version
		Lookup: func(_ string) (io.ReadCloser, error) {
			return nil, errors.ErrUnsupported
		},
	}

	_, err = injector.typeCheck(fset, []parse.File{{Name: "main.go", AstFile: astFile}})
	require.ErrorContains(t, err, "please reinstall and pin orchestrion with a newer Go version")
}
