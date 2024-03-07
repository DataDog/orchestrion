---
title: Built-in Configuration
weight: 10
prev: /docs/getting-started
next: /docs/troubleshooting
---

Orchestrion includes built-in configuration that automatically instruments many libraries using the
Datadog tracing library, {{<godoc "gopkg.in/DataDog/dd-trace-go.v1">}}.

These automated instrumentations are modeled as _aspects_, which are the combination of:
- a _join point_, which is a standardized description of the location where instrumentation code is
  to be added,
- one or more _advice_, which describe the modifications to be made.

The following pages highlights what libraries are supported, how instrumentation is achieved, and
any caveats or limitations to be aware of:

{{<menu icon="document-add">}}
