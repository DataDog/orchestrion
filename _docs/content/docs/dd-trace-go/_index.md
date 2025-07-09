---
title: Datadog Tracer
weight: 80
prev: /docs/getting-started
next: /docs/dd-trace-go/integrations
---

## Default configuration

Orchestrion is complemented by the Datadog tracing library,
{{<godoc import-path="github.com/DataDog/dd-trace-go/v2">}}. It provides
compile-time integrations for many popular Go libraries; and is enabled by
default when running `orchestrion pin`.

The integrations being loaded are configured by your project's root
`orchestrion.tool.go` file, which `orchestrion pin` initializes to something
looking like this:

```go
//go:build tools

//go:generate go run github.com/DataDog/orchestrion pin

package tools

// Imports in this file determine which tracer integrations are enabled in
// orchestrion. New integrations can be automatically discovered by running
// `orchestrion pin` again. You can also manually add new imports here to
// enable additional integrations. When doing so, you can run `orchestrion pin`
// to make sure manually added integrations are valid (i.e, the imported package
// includes a valid `orchestrion.yml` file).
import (
	// Ensures `orchestrion` is present in `go.mod` so that builds are repeatable.
	// Do not remove.
	_ "github.com/DataDog/orchestrion"

	// Provides integrations for essential `orchestrion` features. Most users
	// should not remove this integration.
	_ "github.com/DataDog/dd-trace-go/orchestrion/all/v2" // integration
)
```

## Choosing integrations

Once `orchestrion pin` has been run, you can replace the import of
{{<godoc import-path="github.com/DataDog/dd-trace-go/orchestrion/all/v2">}} with
imports for specific integration packages (see the [Integrations](./v2) section
for a list of available packages).

For example, the below only activates integrations for the core tracer library,
as well as `net/http` clients and servers:

```go
//go:build tools

//go:generate go run github.com/DataDog/orchestrion pin

package tools

// Imports in this file determine which tracer integrations are enabled in
// orchestrion. New integrations can be automatically discovered by running
// `orchestrion pin` again. You can also manually add new imports here to
// enable additional integrations. When doing so, you can run `orchestrion pin`
// to make sure manually added integrations are valid (i.e, the imported package
// includes a valid `orchestrion.yml` file).
import (
	// Ensures `orchestrion` is present in `go.mod` so that builds are repeatable.
	// Do not remove.
	_ "github.com/DataDog/orchestrion"

	// Provides integrations for essential `orchestrion` features. Most users
	// should not remove this integration.
	_ "gopkg.in/DataDog/dd-trace-go.v1/ddtrace/tracer"   // integration
	_ "gopkg.in/DataDog/dd-trace-go.v1/contrib/net/http" // integration
)
```
