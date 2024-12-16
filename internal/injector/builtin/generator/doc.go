// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package main

import (
	"bytes"
	"embed"
	"errors"
	"fmt"
	html "html/template"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"runtime"
	"strings"
	"sync"
	"text/template"
	"unicode"

	"github.com/Masterminds/sprig"
	"golang.org/x/tools/go/packages"
	"gopkg.in/yaml.v3"
)

var (
	//go:embed "doc.*.tmpl"
	htmlTemplateFS embed.FS
	//go:embed "doc.tmpl"
	mdTemplateText string

	once         sync.Once
	mdTemplate   *template.Template
	htmlTemplate *html.Template
)

func documentConfiguration(dir, yamlFile string, config *ConfigurationFile) (string, error) {
	once.Do(func() {
		mdTemplate = template.Must(template.New("").Funcs(template.FuncMap{
			"frontMatter": renderFrontMatter,
			"render": func(v any) (string, error) {
				res, err := renderHTML(v)
				return string(res), err
			},
			"trim": sprig.FuncMap()["trim"],
		}).Parse(mdTemplateText))
		htmlTemplate = html.Must(html.New("").Funcs(html.FuncMap{
			"packageName": resolvePackageName,
			"render":      renderHTML,
			"safe":        func(s string) html.HTML { return html.HTML(s) },
			"tabIndent":   tabIndent,
		}).ParseFS(htmlTemplateFS, "doc.*.tmpl"))
	})

	buf := bytes.NewBuffer(nil)
	if err := mdTemplate.Execute(buf, config); err != nil {
		return "", err
	}

	ext := filepath.Ext(yamlFile)
	filename := filepath.Join(dir, fmt.Sprintf("%s.md", yamlFile[:len(yamlFile)-len(ext)]))
	if err := os.MkdirAll(filepath.Dir(filename), 0o755); err != nil {
		return filename, err
	}
	return filename, os.WriteFile(filename, buf.Bytes(), 0o644)
}

func renderFrontMatter(cfg *ConfigurationFile) (string, error) {
	frontMatter := struct {
		Title string
		Icon  string
	}{
		Title: cfg.Metadata.Name,
		Icon:  cfg.Metadata.Icon,
	}

	var node yaml.Node
	if err := node.Encode(frontMatter); err != nil {
		return "", err
	}
	node.HeadComment = "This file is generated by `go generate ./internal/injector/builtin`. DO NOT EDIT."

	bytes, err := yaml.Marshal(&node)
	return strings.TrimSpace(string(bytes)), err
}

func renderHTML(val any) (html.HTML, error) {
	ns, name := resolveName(val)
	if ns == "" || name == "" {
		return "", fmt.Errorf("%T: %w", val, errors.ErrUnsupported)
	}

	templateName := "doc."
	switch ns {
	case "github.com/DataDog/orchestrion/internal/injector/aspect/join":
		templateName += "join"
	case "github.com/DataDog/orchestrion/internal/injector/aspect/advice":
		templateName += "advice"
	case "github.com/DataDog/orchestrion/internal/injector/aspect/advice/code":
		templateName += "code"
	default:
		return "", fmt.Errorf("%s: %w", ns, errors.ErrUnsupported)
	}

	templateName += "."
	templateName += camelCaseToKebabCase(name)
	templateName += ".tmpl"

	var buf bytes.Buffer
	if err := htmlTemplate.ExecuteTemplate(&buf, templateName, val); err != nil {
		return "", err
	}

	return html.HTML(buf.String()), nil
}

func camelCaseToKebabCase(s string) string {
	result := make([]rune, 0, len(s)+len(s)/2)

	for _, r := range s {
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

func resolveName(val any) (string, string) {
	v := reflect.ValueOf(val)
	if v.Kind() == reflect.Ptr {
		v = v.Elem()
	}

	t := v.Type()
	return t.PkgPath(), t.Name()
}

var resolvePackageNameCache = make(map[string]string)

func resolvePackageName(path string) (string, error) {
	if path == "" || path == "unsafe" {
		return path, nil
	}

	if name, found := resolvePackageNameCache[path]; found {
		return name, nil
	}

	_, thisFile, _, _ := runtime.Caller(0)
	rootDir := filepath.Join(thisFile, "..", "..", "..", "..", "..")

	pkgs, err := packages.Load(&packages.Config{Mode: packages.NeedName, Dir: rootDir}, path)
	if err != nil {
		return "", err
	}
	pkg := pkgs[0]
	if len(pkg.Errors) > 0 {
		return "", pkg.Errors[0]
	}

	resolvePackageNameCache[path] = pkg.Name
	return pkg.Name, nil
}

var indentRe = regexp.MustCompile(`(?m)^(  )+`)

func tabIndent(s string) string {
	return indentRe.ReplaceAllStringFunc(s, func(m string) string {
		return strings.Repeat("\t", len(m)/2)
	})
}
