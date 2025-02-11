---
title: Integrations
weight: 10
prev: /docs/dd-trace-go
next: /docs/dd-trace-go/features
---

Each integration's documentation page provides information on how to enable only this integration,
which can be done by removing the import of
{{<godoc import-path="gopkg.in/DataDog/dd-trace-go.v1">}} from the `orchestrion.tool.go` file, and
replacing it with one or more specific package imports as specified in the documentation.

These compile-time integrations are modeled as _aspects_, which are the combination of:
- a _join point_, which is a standardized description of the location where instrumentation code is
  to be added,
- one or more _advice_, which describe the modifications to be made.

{{<menu icon="document-add">}}
