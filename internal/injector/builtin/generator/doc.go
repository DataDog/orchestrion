// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package main

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"text/template"

	_ "embed"
)

var (
	//go:embed "_doc.tmpl"
	tmplRoot string

	tmpl = template.Must(template.New("").Parse(tmplRoot))
)

func documentConfiguration(dir, yamlFile string, config *ConfigurationFile) error {
	buf := bytes.NewBuffer(nil)

	if err := tmpl.Execute(buf, config); err != nil {
		return err
	}

	ext := filepath.Ext(yamlFile)
	filename := filepath.Join(dir, fmt.Sprintf("%s.md", yamlFile[:len(yamlFile)-len(ext)]))
	if err := os.MkdirAll(filepath.Dir(filename), 0o755); err != nil {
		return err
	}
	return os.WriteFile(filename, buf.Bytes(), 0o644)
}

func writeFmt(buf io.Writer, format string, args ...any) {
	if _, err := fmt.Fprintf(buf, format, args...); err != nil {
		panic(err)
	}
}

func writeLine(buf io.Writer, line string) {
	if _, err := fmt.Fprintln(buf, line); err != nil {
		panic(err)
	}
}
