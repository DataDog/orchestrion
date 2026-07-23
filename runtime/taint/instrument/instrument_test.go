// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package instrument

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/DataDog/orchestrion/runtime/taint"
)

func Test_InstrumentedProgramReportsTaintedOpenPath_when_SourceCrossesLanguageOperations(t *testing.T) {
	// Given
	root := repositoryRoot(t)
	module := t.TempDir()
	writeFixture(t, module, "go.mod", `module example.com/iast-e2e

go 1.25.0

require github.com/DataDog/orchestrion v0.0.0

replace github.com/DataDog/orchestrion => `+root+`
`)
	writeFixture(t, module, "orchestrion.tool.go", `//go:build tools

package tools

import _ "github.com/DataDog/orchestrion/runtime/taint/instrument"
`)
	writeFixture(t, module, "main.go", `package main

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type Path string
type Data []byte
type MoreData []byte
type Octet byte
type Octets []Octet

const constantPath = "constant-" + "path"

func genericJoin[T ~string](left, right T) T {
	return left + right
}

func genericGrow[S ~[]E, E ~byte](value S, element E) S {
	return append(value, element)
}

func shadowedAppend(source []byte) []byte {
	append := func(_ []byte, _ ...byte) []byte { return []byte("clean") }
	return append(nil, source...)
}

func mixedAppend(destination Data, source MoreData) Data {
	return append(destination, source...)
}

func namedStringAppend(destination Data, source Path) Data {
	return append(destination, source...)
}

func main() {
	source := os.Getenv("INPUT")
	joined := "safe-" + source
	data := []byte(joined)
	window := data[5:len(data)]
	copied := make([]byte, len(window))
	copy(copied, window)
	copied = append([]byte{}, copied...)
	path := string(copied)
	_, _ = os.Open(path[0:len(path)])

	_, _ = os.Open("clone-" + strings.Clone(source))
	_, _ = os.Open(strings.Replace("replace-value", "value", source, 1))

	var builder strings.Builder
	builder.WriteString("builder-")
	builder.WriteString(source)
	_, _ = os.Open(builder.String())

	clonedBytes := bytes.Clone([]byte(source))
	_, _ = os.Open(string(append([]byte("bytes-"), clonedBytes...)))
	_, _ = os.Open(fmt.Sprintf("fmt-%s", source))
	_, _ = os.Open(strings.Join([]string{"join", source}, "-"))
	_, _ = os.Open(filepath.Join("/tmp", source))

	var buffer bytes.Buffer
	buffer.WriteString("buffer-")
	buffer.WriteString(source)
	_, _ = os.Open(buffer.String())

	fixedAppend := append([]byte(source), '!')
	_, _ = os.Open(string(fixedAppend))
	stringAppend := append([]byte("append-"), source...)
	_, _ = os.Open(string(stringAppend))
	stringCopy := make([]byte, len(source))
	copy(stringCopy, source)
	_, _ = os.Open("copy-" + string(stringCopy))

	named := Path("named-") + Path(source)
	namedBytes := Data(named)
	_, _ = os.Open(string(Path(namedBytes)))
	_, _ = os.Open(string(genericJoin(Path("generic-"), Path(source))))
	_, _ = os.Open("scalar-" + string(source[0]))
	indexedBytes := []byte(source)
	_, _ = os.Open("byte-" + string(indexedBytes[0]))
	assigned := []byte("x")
	assigned[0] = indexedBytes[0]
	_, _ = os.Open("assigned-" + string(assigned))
	runes := []rune(source)
	_, _ = os.Open("rune-" + string(runes))
	genericBytes := genericGrow(Data(source), byte('!'))
	_, _ = os.Open("generic-bytes-" + string(genericBytes))
	_, _ = os.Open(fmt.Sprintf("map-%v", map[string]string{"value": source}))
	octets := Octets(source)
	_, _ = os.Open("octets-" + string(Path(octets)))
	_, _ = os.Open("target-" + string(Path(source[0])))
	localByte := source[0]
	_, _ = os.Open("local-byte-" + string(localByte))
	localRune := []rune(source)[0]
	_, _ = os.Open("local-rune-" + string(localRune))
	typedClone := bytes.Clone(Data("x"))
	var _ *[]byte = &typedClone
	_, _ = os.Open(string(shadowedAppend([]byte(source))))
	_, _ = os.Open(string(mixedAppend(Data("mixed-"), MoreData(source))))
	_, _ = os.Open(string(namedStringAppend(Data("named-append-"), Path(source))))
	namedCopy := make(Data, len(source))
	copy(namedCopy, Path(source))
	_, _ = os.Open("named-copy-" + string(namedCopy))
	_ = constantPath

	var shifting bytes.Buffer
	shifting.WriteString("x")
	shifting.WriteString(source)
	shifting.Next(1)
	_, _ = os.Open("next-" + shifting.String())
	_, _ = os.Open("repeat-" + strings.Repeat(source, 2))
	_, _ = os.Open("upper-" + strings.ToUpper(source))
	_, _ = os.Open("replace-all-" + strings.ReplaceAll(source, "e", "E"))

	var byteBuffer bytes.Buffer
	byteBuffer.Write([]byte(source))
	_, _ = os.Open("view-" + string(byteBuffer.Bytes()))
	byteBuffer.Truncate(3)
	_, _ = os.Open("truncate-" + byteBuffer.String())
	byteBuffer.Reset()
	byteBuffer.WriteString("clean")
	_, _ = os.Open(byteBuffer.String())
	constructed := bytes.NewBufferString(source)
	_, _ = os.Open("constructed-" + constructed.String())
	aliasBuffer := bytes.NewBufferString("x")
	aliasView := aliasBuffer.Bytes()
	aliasSource := []byte(source)
	aliasView[0] = aliasSource[0]
	_, _ = os.Open("alias-" + aliasBuffer.String())
	cleanAlias := bytes.NewBufferString(source[0:1])
	cleanAlias.Bytes()[0] = 'X'
	_, _ = os.Open(cleanAlias.String())

	cleared := []byte(source)
	clear(cleared)
	_, _ = os.Open(string(cleared))
	overwritten := []byte(source[0:1])
	overwritten[0] = 'X'
	_, _ = os.Open(string(overwritten))

	_, _ = os.Open("secret")
}
`)

	orchestrion := filepath.Join(t.TempDir(), "orchestrion")
	runCommand(t, root, nil, 2*time.Minute, "go", "build", "-o", orchestrion, ".")
	runCommand(t, module, nil, 2*time.Minute, "go", "mod", "tidy")

	// When
	program := filepath.Join(t.TempDir(), "iast-e2e")
	runCommand(t, module, nil, 2*time.Minute, orchestrion, "go", "build", "-p=1", "-o", program, ".")
	output := runCommand(t, module, []string{"INPUT=secret", "ORCHESTRION_TAINT_INCLUDE_VALUE=1"}, 5*time.Second, program)

	// Then
	reports := parseReports(t, output)
	expected := map[string]bool{
		"secret":                false,
		"clone-secret":          false,
		"replace-secret":        false,
		"builder-secret":        false,
		"bytes-secret":          false,
		"fmt-secret":            false,
		"join-secret":           false,
		"/tmp/secret":           false,
		"buffer-secret":         false,
		"secret!":               false,
		"append-secret":         false,
		"copy-secret":           false,
		"named-secret":          false,
		"generic-secret":        false,
		"scalar-s":              false,
		"byte-s":                false,
		"assigned-s":            false,
		"rune-secret":           false,
		"generic-bytes-secret!": false,
		"map-map[value:secret]": false,
		"octets-secret":         false,
		"target-s":              false,
		"local-byte-s":          false,
		"local-rune-s":          false,
		"mixed-secret":          false,
		"named-append-secret":   false,
		"named-copy-secret":     false,
		"next-secret":           false,
		"repeat-secretsecret":   false,
		"upper-SECRET":          false,
		"replace-all-sEcrEt":    false,
		"view-secret":           false,
		"truncate-sec":          false,
		"constructed-secret":    false,
		"alias-s":               false,
	}
	if len(reports) != len(expected) {
		t.Fatalf("reports = %#v, want values %#v", reports, expected)
	}
	for _, report := range reports {
		if report.Sink != "os.Open" || len(report.Ranges) == 0 {
			t.Fatalf("report = %#v", report)
		}
		if report.Value == "next-secret" &&
			(len(report.Ranges) != 1 || report.Ranges[0] != (taint.Range{Start: 5, End: 11})) {
			t.Fatalf("next report ranges = %#v, want [5,11)", report.Ranges)
		}
		if _, exists := expected[report.Value]; !exists {
			t.Fatalf("unexpected report = %#v", report)
		}
		expected[report.Value] = true
	}
	for value, found := range expected {
		if !found {
			t.Errorf("missing report for %q", value)
		}
	}
}

func repositoryRoot(t *testing.T) string {
	t.Helper()
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller() failed")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(filename), "..", "..", ".."))
}

func writeFixture(t *testing.T, directory, name, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(directory, name), []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", name, err)
	}
}

func runCommand(t *testing.T, directory string, environment []string, timeout time.Duration, name string, arguments ...string) string {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	command := exec.CommandContext(ctx, name, arguments...)
	command.Dir = directory
	command.Env = append(os.Environ(), environment...)
	var output bytes.Buffer
	command.Stdout = &output
	command.Stderr = &output
	if err := command.Run(); err != nil {
		if ctx.Err() != nil {
			t.Fatalf("%s %s: %v after %s\n%s", name, strings.Join(arguments, " "), ctx.Err(), timeout, output.String())
		}
		t.Fatalf("%s %s: %v\n%s", name, strings.Join(arguments, " "), err, output.String())
	}
	return output.String()
}

func parseReports(t *testing.T, output string) []taint.Report {
	t.Helper()
	var reports []taint.Report
	for line := range strings.SplitSeq(output, "\n") {
		if !strings.HasPrefix(line, `{"sink":`) {
			continue
		}
		var report taint.Report
		if err := json.Unmarshal([]byte(line), &report); err != nil {
			t.Fatalf("decode report %q: %v", line, err)
		}
		reports = append(reports, report)
	}
	return reports
}
