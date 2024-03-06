// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package processors

import (
	"bufio"
	"fmt"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/datadog/orchestrion/internal/toolexec/proxy"
	"github.com/datadog/orchestrion/internal/toolexec/utils"
)

type PkgRegister struct {
	BuildDir   string
	ImportPath string
	PkgFile    map[string]string
	ImportMap  map[string]string
	RandomData []string
}

func newPkgReg(importPath, buildDir string) PkgRegister {
	return PkgRegister{
		BuildDir:   buildDir,
		ImportPath: importPath,
		PkgFile:    make(map[string]string),
		ImportMap:  make(map[string]string),
		RandomData: make([]string, 0),
	}
}

// Combine merges the two package registers
// In case of conflict on package name, entries from lhs are kept
func (r *PkgRegister) Combine(r2 PkgRegister) {
	for k, v := range r2.ImportMap {
		if _, ok := r.ImportMap[k]; !ok {
			r.ImportMap[k] = v
		}

	}
	for k, v := range r2.PkgFile {
		if _, ok := r.PkgFile[k]; !ok {
			r.PkgFile[k] = v
		}
	}
}

// Import imports the r2 package into r.
// It effectively combines both packages and adds a dependency on r2 in r
func (r *PkgRegister) Import(r2 PkgRegister) {
	r.Combine(r2)
	r.PkgFile[r2.ImportPath] = fmt.Sprintf("%s/b001/_pkg_.a", r2.BuildDir)
}

func (r *PkgRegister) Dump() string {
	var str strings.Builder

	for name, path := range r.ImportMap {
		str.WriteString(fmt.Sprintf("importmap %s=%s\n", name, path))
	}

	for name, path := range r.PkgFile {
		str.WriteString(fmt.Sprintf("packagefile %s=%s\n", name, path))
	}

	for _, data := range r.RandomData {
		str.WriteString(fmt.Sprintf("%s\n", data))
	}

	return str.String()
}

func pkgRegiterFromImportCfg(cfg *os.File) PkgRegister {
	reg := newPkgReg("", filepath.Dir(cfg.Name()))
	scanner := bufio.NewScanner(cfg)
	scanner.Split(bufio.ScanLines)

	for scanner.Scan() {
		line := scanner.Text()
		if line == "" || line[0] == '#' {
			continue
		}
		fields := strings.Fields(line)

		if len(fields) < 2 {
			reg.RandomData = append(reg.RandomData, line)
			continue
		}
		split := strings.Split(fields[1], "=")
		switch fields[0] {
		case "packagefile":
			reg.PkgFile[split[0]] = split[1]
		case "importmap":
			reg.ImportMap[split[0]] = split[1]
		default:
			reg.RandomData = append(reg.RandomData, line)
		}
	}

	return reg
}

// PreparePackage builds the Go package in pkgDir and returns the
// pkgReg including all dependencies and importmaps
// This is aimed at library packages that don't yield and importcfg.link in their b001
// compilation subtree
func PreparePackage(importPath, pkgDir string, buildFlags ...string) (*PkgRegister, error) {
	// 1 - Build pkg
	log.Printf("====> Building %s", importPath)
	wDir, err := utils.GoBuild(pkgDir, buildFlags...)
	if err != nil {
		return nil, err
	}

	pkgReg := newPkgReg(importPath, wDir)

	// 2 - Fetch and combine all dependencies
	log.Printf("====> Building pkg register for %s", importPath)
	filepath.WalkDir(wDir, func(path string, d fs.DirEntry, err error) error {
		if d.IsDir() || d.Name() != "importcfg" {
			return nil
		}
		file, err := os.Open(path)
		if err != nil {
			return err
		}
		pkgReg.Combine(pkgRegiterFromImportCfg(file))
		defer file.Close()
		return nil
	})

	return &pkgReg, err
}

type PackageInjector struct {
	importPath string
	pkgDir     string
	buildFlags []string
}

// NewPackageInjector creates a new package injector
func NewPackageInjector(importPath, pkgDir string, flags ...string) PackageInjector {
	return PackageInjector{
		importPath: importPath,
		pkgDir:     pkgDir,
		buildFlags: flags,
	}
}

// ProcessCompile visits a compile command, compiles the injected package
// and includes the package dependency in the target package's importcfg
func (i *PackageInjector) ProcessCompile(cmd *proxy.CompileCommand) {
	if cmd.Stage() != "b001" {
		return
	}
	log.Printf("[%s] Injecting %s at compile", cmd.Stage(), i.importPath)
	// 1 - Build the package
	pkgReg, err := PreparePackage(i.importPath, i.pkgDir, i.buildFlags...)
	utils.ExitIfError(err)
	state := State{
		Deps: map[string]PkgRegister{i.importPath: *pkgReg},
	}

	// 2 - Add pkg dependency in importcfg
	log.Printf("====> Injecting %s in final importcfg", i.importPath)
	err = filepath.WalkDir(filepath.Dir(cmd.Flags.Output), func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			log.Printf("err at entry: %v", err)
			return err
		}
		if d.IsDir() || d.Name() != "importcfg" {
			return nil
		}

		file, err := os.OpenFile(path, os.O_APPEND|os.O_RDWR|os.O_CREATE, 0640)
		if err != nil {
			log.Printf("err browsing file dir: %v", err)
			return err
		}
		defer file.Close()
		str := fmt.Sprintf("packagefile %s=%s/b001/_pkg_.a", i.importPath, pkgReg.BuildDir)
		_, err = file.WriteString(str)
		return err
	})

	// 3 - Save state to disk for the link invocation (separate process)
	utils.ExitIfError(state.SaveToFile(ddStateFilePath))
	log.Printf("====> Saved state to %s", ddStateFilePath)
}

func (i *PackageInjector) ProcessLink(cmd *proxy.LinkCommand) {
	if cmd.Stage() != "b001" {
		return
	}
	log.Printf("[%s] Injecting %s at link", cmd.Stage(), i.importPath)

	// 1 - Read state from disk (created by ProcessCompile step)
	log.Printf("====> Reading state from %s", ddStateFilePath)
	state, err := StateFromFile(ddStateFilePath)
	defer os.Remove(ddStateFilePath)
	utils.ExitIfError(err)

	// 2 - Process importcfg.link
	file, err := os.Open(cmd.Flags.ImportCfg)
	utils.ExitIfError(err)

	reg := pkgRegiterFromImportCfg(file)

	for _, r := range state.Deps {
		reg.Import(r)
	}

	reg.ImportMap = nil
	file.Close()
	log.Printf("====> Injecting dependencies in importcfg.link")
	file, err = os.Create(cmd.Flags.ImportCfg)
	utils.ExitIfError(err)
	defer file.Close()
	file.WriteString(reg.Dump())
}
