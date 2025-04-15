// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package main

import (
	"bytes"
	"errors"
	"fmt"
	"html/template"
	"path/filepath"
	"reflect"
	"regexp"
	"runtime"
	"strings"
	"unicode"

	"golang.org/x/tools/go/packages"

	"github.com/DataDog/orchestrion/internal/injector/aspect/advice"
	"github.com/DataDog/orchestrion/internal/injector/aspect/advice/code"
	"github.com/DataDog/orchestrion/internal/injector/aspect/join"
	"github.com/DataDog/orchestrion/internal/injector/typed"
)

var (
	_, thisFile, _, _ = runtime.Caller(0)
	docsDir           = filepath.Join(thisFile, "..", "..")

	packageNames = make(map[string]string)
)

func packageName(pkgPath string) (name string, err error) {
	if pkgPath == "" || pkgPath == "unsafe" {
		return pkgPath, nil
	}

	if name, found := packageNames[pkgPath]; found {
		return name, nil
	}

	pkgs, err := packages.Load(&packages.Config{Dir: docsDir, Mode: packages.NeedName}, pkgPath)
	if err != nil {
		return "", fmt.Errorf("packageName %s: %w", pkgPath, err)
	}
	pkg := pkgs[0]
	if len(pkg.Errors) > 0 {
		var err error
		for _, e := range pkg.Errors {
			err = errors.Join(err, e)
		}
		return "", fmt.Errorf("packageName %s: %w", pkgPath, err)
	}

	packageNames[pkgPath] = pkg.Name
	return pkg.Name, nil
}

func render(val any) (template.HTML, error) {
	rv := reflect.ValueOf(val)
	rt := rv.Type()
	if rt.Kind() == reflect.Ptr {
		rt = rt.Elem()
	}

	templateName := "doc."
	switch val := val.(type) {
	case join.Point, typed.TypeName, join.FunctionOption:
		templateName += "join"
	case advice.Advice:
		templateName += "advice"
	case *code.Template:
		templateName += "code"
	default:
		return "", fmt.Errorf("type %T: %w", val, errors.ErrUnsupported)
	}
	templateName += "." + camelToKebab(rt.Name()) + ".tmpl"

	var buf bytes.Buffer
	if err := template.Must(templates.Clone()).
		ExecuteTemplate(&buf, templateName, val); err != nil {
		return "", err
	}

	return template.HTML(buf.String()), nil
}

var indentRe = regexp.MustCompile(`(?m)^(  )+`)

func tabIndent(s string) string {
	return indentRe.ReplaceAllStringFunc(s, func(m string) string {
		return strings.Repeat("\t", len(m)/2)
	})
}

func camelToKebab(text string) string {
	result := make([]rune, 0, len(text)+len(text)/2)

	for _, r := range text {
		if unicode.IsUpper(r) {
			if len(result) > 0 {
				result = append(result, '-')
			}
			r = unicode.ToLower(r)
		}
		result = append(result, r)
	}

	return string(result)
}
