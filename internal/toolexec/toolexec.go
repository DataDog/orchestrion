// Package toolexec acts as a proxy between a Go build command invocation (asm, compile, link...) and its execution.
// It allows inspecting and modifying build commands using a visitor pattern, by defining to main data types:
// - Command, an interface representing a single go toolchain command and all its arguments
// - Injector, an interface that allows injecting (visiting) commands with any kind of new or modified data
package toolexec

import (
	"github.com/datadog/orchestrion/internal/toolexec/processors"
	"github.com/datadog/orchestrion/internal/toolexec/proxy"
	"log"
	"os"
)

var (
	appsecPkg = struct {
		importPath string
		pkgDir     string
	}{
		importPath: "gopkg.in/DataDog/dd-trace-go.v1/internal/appsec",
		pkgDir:     "/Users/francois.mazeau/go/src/github.com/DataDog/dd-trace-go/internal/appsec",
	}

	oldMain = "/Users/francois.mazeau/go/src/github.com/DataDog/instrumentation/main.go"
	newMain = "/Users/francois.mazeau/go/src/github.com/DataDog/instrumentation/.customMain/main.go"
)

// Run executes instrumentation of an application at compile time.
// It works by inspecting and modifying a go tool command invocation.
// args must be the stripped of the program's invocation and any extra orchestrion flag, i.e. it
// must be the raw command line intercepted from the current go build step.
// XXX: future versions will probably need be passed a configuration of sorts to describe which packages/files
// need to be injected/instrumented
func Run(args []string) {
	log.SetFlags(0)
	log.SetPrefix("[instrumentation] | ")
	log.SetOutput(os.Stderr)

	pkgInjector := processors.NewPackageInjector(appsecPkg.importPath, appsecPkg.pkgDir)
	mainSwapper := processors.NewGoFileSwapper(map[string]string{oldMain: newMain})

	cmd := proxy.MustParseCommand(args)
	proxy.ProcessCommand(cmd, mainSwapper.ProcessCompile)
	proxy.ProcessCommand(cmd, pkgInjector.ProcessCompile)
	proxy.ProcessCommand(cmd, pkgInjector.ProcessLink)
	proxy.MustRunCommand(cmd)
	os.Exit(0)
}
