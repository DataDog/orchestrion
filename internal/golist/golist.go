// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

// Package golist contains the types corresponding to the output produced by
// `go list -json`. These structures are copied from `go help list`.
package golist

import (
	"encoding/json"
	"io"
	"time"
)

type Package struct {
	Dir            string   // directory containing package sources
	ImportPath     string   // import path of package in dir
	ImportComment  string   // path in import comment on package statement
	Name           string   // package name
	Doc            string   // package documentation string
	Target         string   // install path
	Shlib          string   // the shared library that contains this package (only set when -linkshared)
	Goroot         bool     // is this package in the Go root?
	Standard       bool     // is this package part of the standard Go library?
	Stale          bool     // would 'go install' do anything for this package?
	StaleReason    string   // explanation for Stale==true
	Root           string   // Go root or Go path dir containing this package
	ConflictDir    string   // this directory shadows Dir in $GOPATH
	BinaryOnly     bool     // binary-only package (no longer supported)
	ForTest        string   // package is only for use in named test
	Export         string   // file containing export data (when using -export)
	BuildID        string   // build ID of the compiled package (when using -export)
	Module         *Module  // info about package's containing module, if any (can be nil)
	Match          []string // command-line patterns matching this package
	DepOnly        bool     // package is only a dependency, not explicitly listed
	DefaultGODEBUG string   // default GODEBUG setting, for main packages

	// Source files
	GoFiles           []string // .go source files (excluding CgoFiles, TestGoFiles, XTestGoFiles)
	CgoFiles          []string // .go source files that import "C"
	CompiledGoFiles   []string // .go files presented to compiler (when using -compiled)
	IgnoredGoFiles    []string // .go source files ignored due to build constraints
	IgnoredOtherFiles []string // non-.go source files ignored due to build constraints
	CFiles            []string // .c source files
	CXXFiles          []string // .cc, .cxx and .cpp source files
	MFiles            []string // .m source files
	HFiles            []string // .h, .hh, .hpp and .hxx source files
	FFiles            []string // .f, .F, .for and .f90 Fortran source files
	SFiles            []string // .s source files
	SwigFiles         []string // .swig files
	SwigCXXFiles      []string // .swigcxx files
	SysoFiles         []string // .syso object files to add to archive
	TestGoFiles       []string // _test.go files in package
	XTestGoFiles      []string // _test.go files outside package

	// Embedded files
	EmbedPatterns      []string // //go:embed patterns
	EmbedFiles         []string // files matched by EmbedPatterns
	TestEmbedPatterns  []string // //go:embed patterns in TestGoFiles
	TestEmbedFiles     []string // files matched by TestEmbedPatterns
	XTestEmbedPatterns []string // //go:embed patterns in XTestGoFiles
	XTestEmbedFiles    []string // files matched by XTestEmbedPatterns

	// Cgo directives
	CgoCFLAGS    []string // cgo: flags for C compiler
	CgoCPPFLAGS  []string // cgo: flags for C preprocessor
	CgoCXXFLAGS  []string // cgo: flags for C++ compiler
	CgoFFLAGS    []string // cgo: flags for Fortran compiler
	CgoLDFLAGS   []string // cgo: flags for linker
	CgoPkgConfig []string // cgo: pkg-config names

	// Dependency information
	Imports      []string          // import paths used by this package
	ImportMap    map[string]string // map from source import to ImportPath (identity entries omitted)
	Deps         []string          // all (recursively) imported dependencies
	TestImports  []string          // imports from TestGoFiles
	XTestImports []string          // imports from XTestGoFiles

	// Error information
	Incomplete bool            // this package or a dependency has an error
	Error      *PackageError   // error loading package
	DepsErrors []*PackageError // errors loading dependencies
}

func ParseJSON(reader io.Reader) ([]*Package, error) {
	var items []*Package
	dec := json.NewDecoder(reader)
	for {
		var item Package
		if err := dec.Decode(&item); err == io.EOF {
			break
		} else if err != nil {
			return nil, err
		}
		items = append(items, &item)
	}

	return items, nil
}

func (p *Package) AllFiles(tests bool) (files []string) {
	files = make([]string, 0,
		len(p.GoFiles)+
			len(p.CgoFiles)+
			len(p.CompiledGoFiles)+
			len(p.IgnoredGoFiles)+
			len(p.IgnoredOtherFiles)+
			len(p.CFiles)+
			len(p.CXXFiles)+
			len(p.MFiles)+
			len(p.HFiles)+
			len(p.FFiles)+
			len(p.SFiles)+
			len(p.SwigFiles)+
			len(p.SwigCXXFiles)+
			len(p.SysoFiles)+
			len(p.TestGoFiles)+
			len(p.XTestGoFiles))

	files = append(files, p.GoFiles...)
	files = append(files, p.CgoFiles...)
	files = append(files, p.CompiledGoFiles...)
	files = append(files, p.IgnoredGoFiles...)
	files = append(files, p.IgnoredOtherFiles...)
	files = append(files, p.CFiles...)
	files = append(files, p.CXXFiles...)
	files = append(files, p.MFiles...)
	files = append(files, p.HFiles...)
	files = append(files, p.FFiles...)
	files = append(files, p.SFiles...)
	files = append(files, p.SwigFiles...)
	files = append(files, p.SwigCXXFiles...)
	files = append(files, p.SysoFiles...)
	if tests {
		files = append(files, p.TestGoFiles...)
		files = append(files, p.XTestGoFiles...)
	}

	return
}

type PackageError struct {
	ImportStack []string // shortest path from package named on command line to this one
	Pos         string   // position of error (if present, file:line:col)
	Err         string   // the error itself
}
type Module struct {
	Path       string       // module path
	Query      string       // version query corresponding to this version
	Version    string       // module version
	Versions   []string     // available module versions
	Replace    *Module      // replaced by this module
	Time       *time.Time   // time version was created
	Update     *Module      // available update (with -u)
	Main       bool         // is this the main module?
	Indirect   bool         // module is only indirectly needed by main module
	Dir        string       // directory holding local copy of files, if any
	GoMod      string       // path to go.mod file describing module, if any
	GoVersion  string       // go version used in module
	Retracted  []string     // retraction information, if any (with -retracted or -u)
	Deprecated string       // deprecation message, if any (with -u)
	Error      *ModuleError // error loading module
	Origin     any          // provenance of module
	Reuse      bool         // reuse of old module info is safe
}

type ModuleError struct {
	Err string // the error itself
}
