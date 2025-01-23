---
title: "Uninstall"
weight: 999

prev: /troubleshooting
---

## Removing Orchestrion

Removing Orchestrion from your project is a simple process to go back to the original state of your project before you
started using Orchestrion.

The steps can be summed up as:
* Remove any files created by orchestrion like `orchestrion.tool.go` and `orchestrion.yml`.
* Run `go mod tidy` to remove any references to orchestrion in your `go.mod` file.
* Remove directives from your source code if any like `//orchestrion:ignore` or `//dd:span`
* Remove any references to orchestrion in your build scripts or CI/CD pipelines or Dockerfile

{{<callout type="info">}}
You can confirm that orchestrion has been removed correctly by looking at your application logs and checking
if they still contain the DataDog Tracer startup log starting with `DATADOG TRACER CONFIGURATION`.
{{</callout>}}
