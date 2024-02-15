// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"

	"github.com/datadog/orchestrion/internal/config"
	"github.com/datadog/orchestrion/internal/injector"
	"github.com/datadog/orchestrion/internal/injector/builtin"
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
	var write bool
	var remove bool
	var httpMode string
	var next bool
	flag.BoolVar(&remove, "rm", false, "remove all instrumentation from the package")
	flag.BoolVar(&write, "w", false, "if set, overwrite the current file with the instrumented file")
	flag.StringVar(&httpMode, "httpmode", "wrap", "set the http instrumentation mode: wrap (default) or report")
	flag.BoolVar(&next, "next", false, "use the next generation code injector")
	printVersion := flag.Bool("v", false, "print orchestrion version")
	flag.Parse()
	if *printVersion {
		fmt.Println(version.Tag)
		return
	}
	if len(flag.Args()) == 0 {
		return
	}

	if next {
		if err := doNext(write, httpMode, flag.Args()...); err != nil {
			fmt.Fprintf(os.Stderr, "An error occurred: %v\n", err)
			os.Exit(1)
		}
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

func doNext(write bool, httpMode string, dirs ...string) error {
	var modifiedFile injector.ModifiedFileFn
	if !write {
		tmp, err := os.MkdirTemp("", "orchestrion")
		if err != nil {
			return fmt.Errorf("failed to create temporary directory: %w", err)
		}
		defer os.RemoveAll(tmp)
		modifiedFile = func(filename string) string {
			return path.Join(tmp, path.Base(filename))
		}
	}

	for _, dir := range dirs {
		gofiles, err := filepath.Glob(path.Join(dir, "*.go"))
		if err != nil {
			return fmt.Errorf("failed to list *.go files in %q: %w", dir, err)
		}

		inj, err := injector.New(dir, injector.Options{
			Aspects:          builtin.Aspects[:],
			ModifiedFile:     modifiedFile,
			PreserveLineInfo: true,
		})
		if err != nil {
			return fmt.Errorf("failed to create injector for %q: %w", dir, err)
		}

		for _, gofile := range gofiles {
			res, err := inj.InjectFile(gofile, map[string]string{"httpmode": httpMode})
			if err != nil {
				return fmt.Errorf("error while injecting code into %q: %w", gofile, err)
			}
			if res.Modified {
				if write {
					fmt.Printf("Injected new code in %q\n", res.Filename)
					if len(res.References) > 0 {
						fmt.Println("New imports have been added, it may be necessary to run `go mod tidy`")
					}
				} else {
					fmt.Printf("Injected new code in %q:\n", gofile)
					data, err := os.ReadFile(res.Filename)
					if err != nil {
						return fmt.Errorf("failed to read modified file %q: %w", res.Filename, err)
					}
					if _, err := os.Stdout.Write(data); err != nil {
						return fmt.Errorf("failed to print modified file content: %w", err)
					}
				}
			}
		}
	}

	return nil
}
