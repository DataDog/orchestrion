---
linkTitle: Documentation
title: Introduction
---

Hello!

Welcome to the Orchestrion documentation!

<!--more-->
## What is Orchestrion?

Orchestrion is a tool that adds Datadog instrumentation to Go applications
automatically at build time. To do so, it uses the standard Go toolchain's
`-toolexec` feature to intercept and possibly modify compilation units before
they are compiled or linked.

## Features

- **Unobtrusive** &ndash; Orchestrion lets developers focus on creating business
  value instead of wasting their time baking observability instrumentation into
  their applications.

- **Exhaustive** &ndash; By running as a `-toolexec` proxy, Orchestrion can not
  only add instrumentation into the application's code; it can also add
  instrumentation into the dependencies' code, including into the Go standard
  library.

- **Flexible** &ndash; Developers can easily influence the observability data
  produced by their applications by adding special directives, such as
  `//orchestrion:ignore`, or `//dd:span custom-tag:value`.

- **Configurable** &ndash; Orchestrion's code manipulations can be configured
  with simple YAML documents, allowing developers to provide specific
  instrumentation configurations for their own frameworks, if Datadog's provided
  configuration does not cover these.

## Questions or Feedback?

{{<callout emoji="❓">}}
  Orchestrion is still under active development, and features and APIs are
  subject to change.

  Have a question or feedback? Feel free to [open an issue][gh-new-issue], or
  engage with us and the community on [GitHub discussions][gh-discussions].

  [gh-new-issue]: https://github.com/DataDog/orchestrion/issues/new/choose
  [gh-discussions]: https://github.com/DataDog/orchestrion/discussions
{{</callout>}}

## Next

Dive right into the following section to get started:

{{<cards>}}
  {{<card
    link="getting-started"
    title="Getting Started"
    icon="play"
    subtitle="Quickly get started with Orchestrion"
  >}}
{{</cards>}}
