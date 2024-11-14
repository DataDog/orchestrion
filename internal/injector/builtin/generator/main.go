// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package main

import (
	_ "embed" // For go:embed
	"errors"
	"flag"
	"fmt"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"text/template"

	"github.com/DataDog/orchestrion/internal/injector/config"
	"github.com/dave/jennifer/jen"
	"gopkg.in/yaml.v3"
)

type globs []string

//go:embed "config.yml.tmpl"
var yamlTemplate string

func main() {
	var (
		pkg           string
		glob          globs
		yamlFile      string
		deps          string
		docsDir       string
		schemadocsDir string
		chomp         int
	)

	flag.StringVar(&pkg, "p", "", "package name")
	flag.Var(&glob, "i", "input files (glob syntax, can be set multiple times)")
	flag.StringVar(&yamlFile, "y", "", "output YAML file")
	flag.StringVar(&deps, "d", "", "dependencies file")
	flag.StringVar(&docsDir, "docs", "", "directory to write documentation files to")
	flag.StringVar(&schemadocsDir, "schemadocs", "", "directory to write schema documentation files to")
	flag.IntVar(&chomp, "C", 0, "number of leading path components to strip from matched file names")
	flag.Parse()

	if len(glob) == 0 {
		log.Fatalln("Missing -i option!")
	}

	if schemadocsDir != "" {
		if err := documentSchema(schemadocsDir); err != nil {
			log.Fatalln(err)
		}
	}

	matches, err := glob.glob()
	if err != nil {
		log.Fatalf("failed to process glob pattern(s) %s: %v\n", glob, err)
	}

	if len(matches) == 0 {
		log.Fatalf("no files matched pattern %q\n", glob)
	}
	// Ensure the files are sorted for determinism.
	sort.Strings(matches)

	if yamlFile != "" {
		file, err := os.Create(yamlFile)
		if err != nil {
			log.Fatalln(err)
		}
		defer file.Close()

		tmpl, err := template.New("").Parse(yamlTemplate)
		if err != nil {
			log.Fatalln(err)
		}
		tmpl.Execute(file, matches)
	}

	docsToDelete := make(map[string]struct{})
	if docsDir != "" {
		filepath.WalkDir(docsDir, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() || filepath.Ext(path) != ".md" || strings.HasPrefix(filepath.Base(path), "_") {
				return nil
			}
			docsToDelete[path] = struct{}{}
			return nil
		})
		defer func() {
			// On exiting, remove no longer needed files
			for path := range docsToDelete {
				if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
					log.Fatalf("Failed to delete old file %q: %v\n", path, err)
				}
			}
		}()
	}

	var depsFile *jen.File
	if deps != "" {
		depsFile = jen.NewFile(pkg)
		depsFile.HeaderComment("Unless explicitly stated otherwise all files in this repository are licensed")
		depsFile.HeaderComment("under the Apache License Version 2.0.")
		depsFile.HeaderComment("This product includes software developed at Datadog (https://www.datadoghq.com/).")
		depsFile.HeaderComment("Copyright 2023-present Datadog, Inc.")
		depsFile.HeaderComment("")
		depsFile.HeaderComment(fmt.Sprintf("Code generated by %q; DO NOT EDIT.", "github.com/DataDog/orchestion/internal/injector/builtin/generator "+strings.Join(os.Args[1:], " ")))
		depsFile.PackageComment("//go:build tools")
	}

	for _, match := range matches {
		config, err := readConfigFile(match)
		if err != nil {
			log.Fatalf("Parsing %q: %v\n", match, err)
		}

		chomped := removeLeadingSegments(match, chomp)
		if docsDir != "" {
			filename, err := documentConfiguration(docsDir, chomped, &config)
			if err != nil {
				log.Fatalf("failed to document aspects from %q: %v\n", match, err)
			}
			delete(docsToDelete, filename)
		}
	}

	if depsFile != nil {
		if err := depsFile.Save(deps); err != nil {
			log.Fatalf("Error writing output file %q: %v\n", deps, err)
		}
	}
}

func readConfigFile(filename string) (ConfigurationFile, error) {
	file, err := os.Open(filename)
	if err != nil {
		return ConfigurationFile{}, err
	}
	defer file.Close()

	var node yaml.Node
	if err := yaml.NewDecoder(file).Decode(&node); err != nil {
		return ConfigurationFile{}, err
	}

	var raw map[string]any
	if err := node.Decode(&raw); err != nil {
		return ConfigurationFile{}, err
	}

	if err := config.ValidateObject(raw); err != nil {
		return ConfigurationFile{}, err
	}

	var result ConfigurationFile
	if err := node.Decode(&result); err != nil {
		return ConfigurationFile{}, err
	}

	return result, nil
}

func init() {
	log.SetPrefix("github.com/DataDog/orchestrion/internal/injector/builtin/generate: ")
}

func (g *globs) Set(value string) error {
	*g = append(*g, value)
	return nil
}

func (g *globs) String() string {
	return strings.Join(*g, " ")
}

func (g *globs) glob() (files []string, err error) {
	unique := make(map[string]struct{})
	for _, pattern := range *g {
		matches, err := filepath.Glob(pattern)
		if err != nil {
			return nil, err
		}
		for _, match := range matches {
			if _, found := unique[match]; found {
				continue
			}
			unique[match] = struct{}{}
			files = append(files, match)
		}
	}

	if len(files) == 0 {
		err = fmt.Errorf("no files matched pattern(s) %s", g)
	}

	// Ensure output is sorted for determinism...
	sort.Strings(files)

	return
}

func removeLeadingSegments(path string, n int) string {
	if n <= 0 {
		return path
	}
	sep := string(filepath.Separator)
	parts := strings.Split(path, sep)
	if len(parts) <= n {
		return path
	}
	return strings.Join(parts[n:], sep)
}
