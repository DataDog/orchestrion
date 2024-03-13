// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package main

import (
	"flag"
	"fmt"
	"github.com/datadog/orchestrion/internal/injector"
	"github.com/datadog/orchestrion/internal/injector/builtin"
	"github.com/datadog/orchestrion/internal/injector/typed"
	"github.com/datadog/orchestrion/internal/toolexec/processors"
	"github.com/datadog/orchestrion/internal/toolexec/proxy"
	"io"
	"log"
	"os"
	"path/filepath"

	"github.com/datadog/orchestrion/internal/config"
	"github.com/datadog/orchestrion/internal/instrument"
	"github.com/datadog/orchestrion/internal/version"
)

func main() {
	flag.Usage = func() {
		w := flag.CommandLine.Output()
		fmt.Fprint(w, "usage: orchestrion [options] [path]\n")
		fmt.Fprint(w, "example: orchestrion -w ./\n")
		fmt.Fprint(w, "options:\n")
		flag.PrintDefaults()
	}
	var (
		write, remove, proxyMode bool
		httpMode                 string
	)
	flag.BoolVar(&remove, "rm", false, "remove all instrumentation from the package")
	flag.BoolVar(&write, "w", false, "if set, overwrite the current file with the instrumented file")
	flag.BoolVar(&proxyMode, "proxy", false, "if set, <description>")
	flag.StringVar(&httpMode, "httpmode", "wrap", "set the http instrumentation mode: wrap (default) or report")
	printVersion := flag.Bool("v", false, "print orchestrion version")
	flag.Parse()
	if *printVersion {
		fmt.Println(version.Tag)
		return
	}

	if len(flag.Args()) == 0 {
		return
	}

	if proxyMode {
		hijack(flag.Args())
		return
	}

	output := func(fullName string, out io.Reader) {
		fmt.Printf("%s:\n", fullName)
		// write the output
		txt, _ := io.ReadAll(out)
		fmt.Println(string(txt))
	}
	if write {
		output = func(fullName string, out io.Reader) {
			fmt.Printf("overwriting %s:\n", fullName)
			// write the output
			txt, _ := io.ReadAll(out)
			err := os.WriteFile(fullName, txt, 0644)
			if err != nil {
				fmt.Printf("Writing file %s: %v\n", fullName, err)
			}
		}
	}
	conf := config.Config{HTTPMode: httpMode}
	if err := conf.Validate(); err != nil {
		fmt.Printf("Config error: %v\n", err)
		os.Exit(1)
	}
	for _, v := range flag.Args() {
		p, err := filepath.Abs(v)
		if err != nil {
			fmt.Printf("Sanitizing path (%s) failed: %v\n", v, err)
			continue
		}
		fmt.Printf("Scanning Package %s\n", p)
		processor := instrument.InstrumentFile
		if remove {
			fmt.Printf("Removing Orchestrion instrumentation.\n")
			processor = instrument.UninstrumentFile
		}
		err = instrument.ProcessPackage(p, processor, output, conf)
		if err != nil {
			fmt.Printf("Failed to scan: %v\n", err)
			os.Exit(1)
		}
	}
}

type orchestrator struct {
	replacedFiles map[string]string
	pkgRegister   processors.PackageRegister
}

func newOrchestrator() *orchestrator {
	return &orchestrator{
		replacedFiles: make(map[string]string),
		refs:          make(typed.ReferenceMap),
	}
}

func hijack(args []string) {
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("Recovered a panic while hijacking:", r)
		}
	}()
	o := newOrchestrator()
	cmd := proxy.MustParseCommand(args)
	proxy.ProcessCommand(cmd, o.injectGoFiles)
	for importPath := range o.refs {
		pkgInjector := processors.NewPackageInjector(importPath, "todo: find source dir")
		proxy.ProcessCommand(cmd, pkgInjector.ProcessCompile)
		proxy.ProcessCommand(cmd, pkgInjector.ProcessLink)
	}
	proxy.MustRunCommand(cmd)
}

func (o *orchestrator) injectGoFiles(cmd *proxy.CompileCommand) {
	i, err := injector.New(cmd.BuildDir, injector.Options{
		Aspects:          builtin.Aspects[:],
		ModifiedFile:     modifiedFileName,
		PreserveLineInfo: true,
	})
	if err != nil {
		panic(err)
	}

	goFiles := cmd.GoFiles()
	for _, f := range goFiles {
		res, err := i.InjectFile(f, map[string]string{"<TBD>": "TBD"})
		if err != nil {
			log.Printf("error while injecting %s, f", f)
			continue
		}
		if !res.Modified {
			continue
		}

		replacedFiles[f] = res.Filename
	}
	log.Printf("replaced files: %v\n", replacedFiles)
	log.Printf("go files: %v\n", goFiles)
}

func modifiedFileName(fileName string) string {
	return fmt.Sprintf("injected_%s", fileName)
}
