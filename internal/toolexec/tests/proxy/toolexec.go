// Package toolexec acts as a proxy between a Go build command invocation (asm, compile, link...) and its execution.
// It allows inspecting and modifying build commands using a visitor pattern, by defining to main data types:
// - Command, an interface representing a single go toolchain command and all its arguments
// - Injector, an interface that allows injecting (visiting) commands with any kind of new or modified data
package main

import (
	"io"
	"log"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v2"

	"github.com/datadog/orchestrion/internal/toolexec/processors"
	"github.com/datadog/orchestrion/internal/toolexec/proxy"
)

type Cfg struct {
	Inject  map[string]string `yaml:"inject,omitempty"`
	Replace map[string]string `yaml:"replace,omitempty"`
}

func cfgFromYaml(path string) (Cfg, error) {
	var cfg Cfg
	yamlFile, err := os.ReadFile(path)
	if err != nil {
		return cfg, err
	}
	if err = yaml.Unmarshal(yamlFile, &cfg); err != nil {
		return cfg, err
	}

	absReplace := make(map[string]string, len(cfg.Replace))
	for src, dst := range cfg.Replace {
		delete(cfg.Replace, src)
		srcAbs, _ := filepath.Abs(src)
		dstAbs, _ := filepath.Abs(dst)
		absReplace[srcAbs] = dstAbs
	}
	cfg.Replace = absReplace
	return cfg, err
}

func main() {
	log.SetFlags(0)
	log.SetOutput(io.Discard)

	args := os.Args[1:]
	if len(args) <= 1 {
		log.Fatalln("Not enough arguments")
	}
	cfg, err := cfgFromYaml(args[0])
	if err != nil {
		log.Fatalf("Failed parsing configuration from %s: %v\n", args[0], err)
	}
	cmd := proxy.MustParseCommand(args[1:])

	if len(cfg.Replace) > 0 {
		swapper := processors.NewGoFileSwapper(cfg.Replace)
		proxy.ProcessCommand(cmd, swapper.ProcessCompile)
	}
	for path, importPath := range cfg.Inject {
		pkgInj := processors.NewPackageInjector(importPath, path)
		proxy.ProcessCommand(cmd, pkgInj.ProcessCompile)
		proxy.ProcessCommand(cmd, pkgInj.ProcessLink)
	}
	proxy.MustRunCommand(cmd)
}
