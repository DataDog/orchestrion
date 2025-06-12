---
title: Feature Activation
weight: 20

prev: /docs/dd-trace-go/integrations
next: /docs/dd-trace-go/custom-trace
---

## Custom Go Tracer start-up

{{<callout type="info">}}
The core tracer integration is provided by:
[`github.com/DataDog/dd-trace-go/v2/ddtrace/tracer`](../v2/ddtrace-tracer)
{{</callout>}}

All applications built using `orchestrion` automatically start the Datadog
tracer at the beginning of the `main` function using the tracer library's
default configuration. The recommended way to configure the tracer is by using
the designated environment variables, such as `DD_ENV`, `DD_SERVICE`,
`DD_VERSION`, etc... You can get more information on what environment variables
are available in the [documentation][env-var-doc].

If the `main` function is annotated with the `//orchestrion:ignore` directive, the tracer
will not be started automatically, and you are responsible for calling
{{<godoc import-path="gopkg.in/DataDog/dd-trace-go.v1/ddtrace/tracer" package="tracer" name="Start" >}}
with your preferred configuration options.

[env-var-doc]: https://docs.datadoghq.com/tracing/trace_collection/library_config/go/#unified-service-tagging

## Enabling the Go Profiler

{{<callout type="info">}}
The Go continuous profiler integration is provided by:
- [`github.com/DataDog/dd-trace-go/v2/profiler`](../v2/profiler)
{{</callout>}}

All applications built using `orchestrion` automatically start the Datadog
continuous profiler if the `DD_PROFILING_ENABLED` environment variable is set
to `1` or `true`. If profiling is enabled via the
[Datadog Admission Controller][dd-adm-controller], `DD_PROFILING_ENABLED` can be
set to `auto`.

When enabled, the continuous profiler will activate the following profiles:
- {{<godoc import-path="github.com/DataDog/dd-trace-go/v2/profiler" package="profiler" name="CPUProfile" >}}
- {{<godoc import-path="github.com/DataDog/dd-trace-go/v2/profiler" package="profiler" name="HeapProfile" >}}
- {{<godoc import-path="github.com/DataDog/dd-trace-go/v2/profiler" package="profiler" name="GoroutineProfile" >}}
- {{<godoc import-path="github.com/DataDog/dd-trace-go/v2/profiler" package="profiler" name="MutexProfile" >}}

[dd-adm-controller]: https://docs.datadoghq.com/containers/cluster_agent/admission_controller/?tab=datadogoperator

## Enabling Application Security features

{{<callout type="info">}}
The Application Security (ASM) features are supported by the core tracer
integrations, and baked into the various supported library integrations. As a
result, ASM requires you enable:
- [`github.com/DataDog/dd-trace-go/v2/ddtrace/tracer`](../v2/ddtrace-tracer)
- Integrations for supported libraries you want monitoring and/or protection for
{{</callout>}}

Datadog Application Security (ASM) features are built into the tracer library,
but need to be enabled at run-time. The [Enabling ASM for Go][asm-for-go]
documentation explains how to enable Application Security for instrumented go
applications.

In the majority of cases, all that's needed is to set `DD_APPSEC_ENABLED` to `1`
or `true`.

{{<callout emoji="⚠️">}}
Datadog's Application Security features are only supported on Linux (AMD64,
ARM64) and macOS (AMD64, ARM64).

On Linux platforms, the [Datadog in-app WAF][libddwaf] needs the `libc.so.6` and
`libpthread.so.0` shared libraries to be available; even if `CGO_ENABLED=1`.

If your are building your applications in environments where `CGO_ENABLED=0`,
Application Security features are only available if you specify the `appsec`
build tag (`orchestrion go build -tags=appsec .`).

For more information, refer to the [Enabling ASM for Go][asm-for-go]
documentation.

[libddwaf]: https://github.com/DataDog/libddwaf
[asm-for-go]: https://docs.datadoghq.com/security/application_security/threats/setup/threat_detection/go/
{{</callout>}}

Building applications with `orchestrion` allows you to maximize coverage for
<abbr title="Runtime Application Self-Protection">RASP</abbr> features, such as
automatic protection against SQL Injection attacks.

[asm-for-go]: https://docs.datadoghq.com/security/application_security/threats/setup/threat_detection/go/
