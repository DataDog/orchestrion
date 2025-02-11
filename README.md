# Orchestrion

[![User Documentation](https://img.shields.io/badge/docs.datadoghq.com-blue?logo=datadog&label=User%20Guide&labelColor=632CA6&style=flat)](https://docs.datadoghq.com/tracing/trace_collection/automatic_instrumentation/dd_libraries/go/?tab=compiletimeinstrumentation)
[![Project Documentation](https://img.shields.io/badge/Project%20Documentation-datadoghq.dev/orchestrion-blue.svg?logo=github&&labelColor=181717&style=flat)](https://datadoghq.dev/orchestrion)
![Latest Release](https://img.shields.io/github/v/release/DataDog/orchestrion?display_name=tag&label=Latest%20Release)
![GitHub go.mod Go version](https://img.shields.io/github/go-mod/go-version/datadog/orchestrion)
[![OpenSSF Scorecard](https://api.scorecard.dev/projects/github.com/DataDog/orchestrion/badge)](https://scorecard.dev/viewer/?uri=github.com/DataDog/orchestrion)

Automatic compile-time instrumentation of Go code.

## Overview

Orchestrion processes Go source code at compilation time and automatically inserts instrumentation. This instrumentation
is driven by the imports present in the `orchestrion.tool.go` file at the project's root.

> [!IMPORTANT]
> Should you encounter issues or a bug when using `orchestrion`, please report it in the [bug tracker][gh-issues].
>
> For support & general questions, you are welcome to use [GitHub discussions][gh-discussions]. You may also contact us
> privately via Datadog support.
>
> [gh-issues]: https://github.com/DataDog/orchestrion/issues/new/choose
> [gh-discussions]: https://github.com/DataDog/orchestrion/discussions

## Requirements

Orchestrion supports the two latest releases of Go, matching Go's [official release policy][go-releases]. It may
function correctly with older Go releases; but we will not be able to offer support for these if they don't.

In addition to this, Orchestrion only supports projects using [Go modules][go-modules].

[go-releases]: https://go.dev/doc/devel/release#policy
[go-modules]: https://pkg.go.dev/cmd/go#hdr-Modules__module_versions__and_more

## Getting started

Information on how to get started quickly with orchestrion can be found on the [user guide][dd-doc-getting-started].

[dd-doc-getting-started]: https://docs.datadoghq.com/tracing/trace_collection/automatic_instrumentation/dd_libraries/go/?tab=compiletimeinstrumentation#overview

## Datadog Tracer Integrations

Importing `gopkg.in/DataDog/dd-trace-go.v1` (version `v1.72.0-rc.1` or later is required) in the project root's
`orchestrion.tool.go` file enables automatic instrumentation of all supported integrations, which are listed on the
[documentation site][docsite]. You can cherry-pick which integrations are enabled by `orchestrion` by importing the
desired integrations' package paths instead of importing the tracer's root module.

> [!TIP]
> Orchestrion is a vendor-agnostic tool. By default, `orchestrion pin` enables Datadog's tracer integrations by
> importing `gopkg.in/DataDog/dd-trace-go.v1` in `orchestrion.tool.go`, but other vendors (such as OpenTelemetry) may
> provide alternate integrations that can be used instead.

[docsite]: https://docs.datadoghq.com/tracing/trace_collection/compatibility/go/?tab=v1

## Troubleshooting

If you run into issues when using `orchestrion` please make sure to collect all relevant details about your setup in
order to help us identify (and ideally reproduce) the issue. The version of orchestrion (which can be obtained from
`orchestrion version`) as well as of the go toolchain (from `go version`) are essential and must be provided with any
bug report.

You can inspect everything Orchestrion is doing by adding the `-work` argument to your `go build` command; when doing so
the build will emit a `WORK=` line pointing to a working directory that is retained after the build is finished. The
contents of this directory contains all updated source code Orchestrion produced and additional metadata that can help
diagnosing issues.

## More information

Datadog's user guide for Orchestrion can be found on [docs.datadoghq.com][dd-doc].

[dd-doc]: https://docs.datadoghq.com/tracing/trace_collection/automatic_instrumentation/dd_libraries/go/?tab=compiletimeinstrumentation

Orchestrion's project documentation can be found at [datadoghq.dev/orchestrion](https://datadoghq.dev/orchestrion); in
particular:
- the [user guide](https://datadoghq.dev/orchestrion/docs/) provides information about available configuration, and how
  to customize the traces produced by your application;
- the [contributor guide](https://datadoghq.dev/orchestrion/contributing/) provides more detailed information about how
  orchestrion works and how to contribute new instrumentation to it.
