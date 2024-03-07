// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package processors

import (
	"bufio"
	"fmt"
	"io"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/datadog/orchestrion/internal/toolexec/proxy"
	"github.com/datadog/orchestrion/internal/toolexec/utils"
)

// PackageRegister describes Go package and its dependencies
// It allows reading and editing the content of an `importcfg` file during a Go build step
type PackageRegister struct {
	SourceDir  string
	ImportPath string
	// PackageFile maps package dependencies fully-qualified import paths to their build archive location
	PackageFile map[string]string
	// ImportMap maps package dependencies import paths to their fully-qualified version
	ImportMap map[string]string
	// RandomData is data read from an `importcfg` file that is not needed
	// We store it so that we can still write the full `importcfg` file back, if needed
	RandomData []string
}

func newPackageRegister(importPath, buildDir string) PackageRegister {
	return PackageRegister{
		SourceDir:   buildDir,
		ImportPath:  importPath,
		PackageFile: make(map[string]string),
		ImportMap:   make(map[string]string),
	}
}

// Combine copies entries from other into the receiver unless the
// receiver already has a package with the same name.
func (r *PackageRegister) Combine(other PackageRegister) {
	for k, v := range other.ImportMap {
		if _, ok := r.ImportMap[k]; !ok {
			r.ImportMap[k] = v
		}

	}
	for k, v := range other.PackageFile {
		if _, ok := r.PackageFile[k]; !ok {
			r.PackageFile[k] = v
		}
	}
}

// Import imports the other package into r.
// It effectively combines both packages and adds a dependency on r2 in r
func (r *PackageRegister) Import(other PackageRegister) {
	r.Combine(other)
	r.PackageFile[other.ImportPath] = fmt.Sprintf("%s/b001/_pkg_.a", other.SourceDir)
}

// WriteTo writes the content of the package register to the provided writer
// Writing to a file will yield a valid importcfg Go build file
func (r *PackageRegister) WriteTo(writer io.Writer) error {
	for name, path := range r.ImportMap {
		if _, err := io.WriteString(writer, fmt.Sprintf("importmap %s=%s\n", name, path)); err != nil {
			return err
		}
	}

	for name, path := range r.PackageFile {
		if _, err := io.WriteString(writer, fmt.Sprintf("packagefile %s=%s\n", name, path)); err != nil {
			return err
		}
	}

	for _, data := range r.RandomData {
		if _, err := io.WriteString(writer, fmt.Sprintf("%s\n", data)); err != nil {
			return err
		}
	}

	return nil
}

func parseImportConfig(cfg *os.File) PackageRegister {
	reg := newPackageRegister("", filepath.Dir(cfg.Name()))
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
			reg.PackageFile[split[0]] = split[1]
		case "importmap":
			reg.ImportMap[split[0]] = split[1]
		default:
			reg.RandomData = append(reg.RandomData, line)
		}
	}

	return reg
}

// BuildPackage builds the Go package in sourceDir and returns the package register holding all
// dependencies and importmaps for that package. This is aimed at library packages that don't
// yield and importcfg.link in their b001 compilation subtree
func BuildPackage(importPath, pkgDir string, buildFlags ...string) (*PackageRegister, error) {
	// 1 - Build pkg
	log.Printf("====> Building %s\n", importPath)
	wDir, err := utils.GoBuild(pkgDir, buildFlags...)
	if err != nil {
		return nil, err
	}

	pkgReg := newPackageRegister(importPath, wDir)

	// 2 - Fetch and combine all dependencies
	log.Printf("====> Building pkg register for %s\n", importPath)
	filepath.WalkDir(wDir, func(path string, d fs.DirEntry, err error) error {
		if d.IsDir() || d.Name() != "importcfg" {
			return nil
		}
		file, err := os.Open(path)
		if err != nil {
			return err
		}
		defer file.Close()
		pkgReg.Combine(parseImportConfig(file))
		return nil
	})

	return &pkgReg, err
}

// PackageInjector holds information needed to inject a Go package
// and all its dependencies into the build process.
type PackageInjector struct {
	importPath string
	sourceDir  string
	buildFlags []string
}

// NewPackageInjector initializes a command processor that will build the package code
// at sourceDir using the provided build flags, and add all resulting imports and dependencies
// into the compilation tree of the current app
func NewPackageInjector(importPath, sourceDir string, flags ...string) PackageInjector {
	return PackageInjector{
		importPath: importPath,
		sourceDir:  sourceDir,
		buildFlags: flags,
	}
}

// ProcessCompile visits a compile command, compiles the injected package
// and includes the package dependency in the target package's importcfg
func (i *PackageInjector) ProcessCompile(cmd *proxy.CompileCommand) {
	if cmd.Stage() != "b001" {
		return
	}
	log.Printf("[%s] Injecting %s at compile\n", cmd.Stage(), i.importPath)
	// 1 - Build the package
	pkgReg, err := BuildPackage(i.importPath, i.sourceDir, i.buildFlags...)
	utils.ExitIfError(err)
	state := State{
		Deps: map[string]PackageRegister{i.importPath: *pkgReg},
	}

	// 2 - Add pkg dependency in importcfg
	log.Printf("====> Injecting %s in final importcfg\n", i.importPath)
	err = filepath.WalkDir(filepath.Dir(cmd.Flags.Output), func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			log.Printf("error at entry: %v\n", err)
			return err
		}
		if d.IsDir() || d.Name() != "importcfg" {
			return nil
		}

		file, err := os.OpenFile(path, os.O_APPEND|os.O_RDWR, 0o640)
		if err != nil {
			log.Printf("error opening %s: %v\n", path, err)
			return err
		}
		defer file.Close()
		str := fmt.Sprintf("packagefile %s=%s/b001/_pkg_.a", i.importPath, pkgReg.SourceDir)
		_, err = file.WriteString(str)
		return err
	})

	// 3 - Save state to disk for the link invocation (separate process)
	utils.ExitIfError(state.SaveToFile(ddStateFilePath))
	log.Printf("====> Saved state to %s\n", ddStateFilePath)
}

// ProcessLink visits a link command and includes all the new package dependencies
// yielded by the compile step in importcfg.link
func (i *PackageInjector) ProcessLink(cmd *proxy.LinkCommand) {
	if cmd.Stage() != "b001" {
		return
	}
	log.Printf("[%s] Injecting %s at link\n", cmd.Stage(), i.importPath)

	// 1 - Read state from disk (created by ProcessCompile step)
	log.Printf("====> Reading state from %s\n", ddStateFilePath)
	state, err := LoadFromFile(ddStateFilePath)
	defer os.Remove(ddStateFilePath)
	utils.ExitIfError(err)

	// 2 - Process importcfg.link
	file, err := os.Open(cmd.Flags.ImportCfg)
	utils.ExitIfError(err)

	reg := parseImportConfig(file)

	for _, r := range state.Deps {
		reg.Import(r)
	}

	reg.ImportMap = nil
	file.Close()
	log.Printf("====> Injecting dependencies in importcfg.link\n")
	file, err = os.Create(cmd.Flags.ImportCfg)
	utils.ExitIfError(err)
	defer file.Close()
	reg.WriteTo(file)
}
