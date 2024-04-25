# Orchestrion

Automatic compile-time instrumentation of Go code

![Orchestrion](https://upload.wikimedia.org/wikipedia/commons/5/55/Welteorchestrion1862.jpg)

## Overview

Orchestrion processes Go source code at compilation time and automatically inserts instrumentation. This instrumentation
produces Datadog APM traces from the instrumented code and supports Datadog Application Security Management. Future work
will include support for OpenTelemetry tracing as well.

## Getting started

1. Install Orchestrion:
    ```console
    $ go install github.com/datadog/orchestrion@latest
    ```

2. <details><summary>Optional: artifact cache warm-up</summary>

      > _Orchestrion_ can modify code in the entire application stack, including the standard library. To avoid this
      > interferes with non-Orchestrion development on the same machine, `orchestrion` uses its own builds of everything.
      > This means the very first `orchestrion`-enabled build you run will fully re-build the Go standard library, and some
      > of Orchestrion's own instrumentation libraries.
      >
      > Orchestrion provides a single command to pre-build the standard library and all instrumentation libraries
      > Orchestrion may inject into compiled code:
      > ```console
      > $ orchestrion warmup
      > ```
      > It is recommended to run this command when building container images (e.g, docker images) that ship with
      > `orchestrion`, as this could significantly improve the performance of builds subsequently performed using these
      > images.
      >
      > The `orchestrion`-specific builds are tied to the specific version of the `go` toolchain being used as well as
      > `orchestrion`'s version. You may want to re-run `orchestrion warmup` after having updated your Orchestrion
      > dependency.
    </details>

3. <details><summary>Optional: project <tt>go.mod</tt> registration</summary>

      >  You can automatically add `orchestrion` to your project's dependencies by running:
      > ```console
      > $ orchestrion pin
      > ```
      > This will:
      > 1. Create a new `orchestrion.tool.go` file containing content similar to:
      >     ```go
      >     // Code generated by `orchestrion pin`; DO NOT EDIT.
      >
      >     // This file is generated by `orchestrion pin`, and is used to include a blank import of the
      >     // orchestrion package(s) so that `go mod tidy` does not remove the requirements from go.mod.
      >     // This file should be checked into source control.
      >
      >     //go:build tools
      >
      >     package tools
      >
      >     import _ "github.com/datadog/orchestrion"
      >     ```
      > 2. Run `go get github.com/datadog/orchstrion@<current-release>` to make sure the project version corresponds to the
      >    one currently being used
      > 3. Run `go mod tidy` to make sure your `go.mod` and `go.sum` files are up-to-date
      >
      > If you do not run this command, it will be done automatically when required. Once done, the version of `orchestrion`
      > used by this project can be controlled directly using the `go.mod` file, as you would control any other dependency.
    </details>

4. Prefix your `go` commands with `orchestrion`:
    ```console
    $ orchestrion go build .
    $ orchestrion go test -race ./...
    ```
    <details><summary>Alternative</summary>

    > _Orchestrion_ at the core is a standard Go toolchain `-toolexec` proxy. Instead of using `orchestrion go`, you can
    > also manually provide the `-toolexec` argument to `go` commands that accept it:
    > ```console
    > $ go build -toolexec 'orchestrion toolexec' .
    > $ go test -toolexec 'orchestrion toolexec' -race .
    > ```
    </details>

> The version of `orchestrion` used to compile your project is ultimately tracked in the `go.mod` file. You can manage
> it in the same way you manage any other dependency, and updating to the latest release is as simple as doing:
> ```console
> $ go get github.com/datadog/orchestrion@latest
> ```

## How it works

The go toolchain's `-toolexec` feature invokes `orchestrion toolexec` with the complete list of arguments for each
toolchain command invocation, allowing `orchestrion` to inspect and modify those before executing the actual command.
This allows `orchestrion` to inspect all the go source files that contribute to the complete application, and to modify
these to include instrumentation code where appropriate. Orchestrion uses [`dave/dst`][dave-dst] to parse and modify the
go source code. Orchestrion adds `//line` directive comments in modified source files to make sure the stack trace
information produced by the final application are not affected by additional code added during instrumentation.

Since the instrumentation may use packages not present in the original code, `orchestrion` also intercepts the standard
go linker command invocations to make the relevant packages available to the linker.

[dave-dst]: https://github.com/dave/dst

### Directive comments

Directive comments are special single-line comments with no space between then `//` and the directive name. These allow
influencing the behavior of Orchestrion in a declarative manner.

#### `//dd:ignore`

The `//dd:ignore` directive instructs Orchestrion not to perform any code changes in Go code nested in the decorated
scope: when applied to a statement, it prevents instrumentations from being added to any component of this statement,
and when applied to a block, or function, it prevents instrumentation from being added anywhere in that block or
function.

This is useful when you specifically want to opt out of instrumenting certain parts of your code, either because it has
already been instrumented manually, or because the tracing is undesirable (not useful, adds too much overhead in a
performance-critical section, etc...).

#### `//dd:span`

Use a `//dd:span` comment before any function or method to create specific spans from your automatically instrumented
code. Spans will include tags described as arguments in the `//dd:span`. In order for the directive to be recognized,
the line-comment must be spelled out with no white space after the `//` comment start.

A function or method annotated with `//dd:span` must receive an argument of type `context.Context` or `*http.Request`.
The context or request is required for trace information to be passed through function calls in a Go program. If this
condition is met, the `//dd:span` comment is scanned and code is inserted in the function preamble (before any other
code).

Span tags are specified as a space-delimited series of `name:value` pairs, or as simple expressions referring to
argument names (or access to fields thereof). All `name:value` pairs are provided as strings, and expressions are
expected to evaluate to strings as well.

```go
//dd:span my:tag type:request name req.Method
func HandleRequest(name string, req *http.Request) {
	// ↓↓↓↓ Instrumentation added by Orchestrion ↓↓↓↓
	req = req.WithContext(instrument.Report(req.Context(), event.EventStart, "function-name", "HandleRequest", "my", "tag", "type", "request", "name", name, "req.Method", req.Method))
	defer instrument.Report(req.Context(), event.EventEnd, "function-name", "HandleRequest", "my", "tag", "type", "request", "name", name, "req.Method", req.Method)
	// ↑↑↑↑ End of added instrumentation ↑↑↑↑

	// your code here
}
```

## Supported libraries

Orchestrion supports automatic tracing of the following libraries:

- `net/http`
- `database/sql`
- `google.golang.org/grpc`
- `github.com/gin-gonic/gin`
- `github.com/labstack/echo/v4`
- `github.com/go-chi/chi/v5`
- `github.com/gorilla/mux`
- `github.com/gofiber/fiber/v2`

Calls to these libraries are instrumented with library-specific code adding tracing to them, including support for
distributed traces.

[1]: https://github.com/DataDog/go-sample-app

## Troubleshooting

If you run into issues when using `orchestrion` please make sure to collect all relevant details about your setup in
order to help us identify (and ideally reproduce) the issue. The version of orchestrion (which can be obtained from
`orchestrion version`) as well as of the go toolchain (from `go version`) are essential and must be provided with any
bug report.

You can inspect everything Orchestrion is doing by adding the `-work` argument to your `go build` command; when doing so
the build will emit a `WORK=` line pointing to a working directory that is retained after the build is finished. The
contents of this directory contains all updated source code Orchestrion produced and additional metadata that can help
diagnosing issues.
