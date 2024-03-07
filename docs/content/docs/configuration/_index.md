---
title: Configuration
weight: 3
draft: true
---

## Introduction

Orchestrion code manipulations are configured using YAML documents. It comes
with built-in defaults that cover available Datadog integrations, such as
instrumenting the `net/http` package. Users of orchestrion can also specify
their own transforms by specifying the `--config <path/to/config.yml>` flag (or
the equivalent `DD_ORCHESTRION_CONFIG` environment variable). This section
provides reference information about the available configuration primitives.

## Aspects

Orchestrion code transformations are called _Aspects_ and are the combination of
a _Join Point_ with one or more _Advices_. The configuration file is composed of
a `meta` block that provides basic documentation information, and a list of
_aspects_:

```yaml
%YAML 1.1
---
meta:
  name: ... # The name of this configuration (for documentation purposes)
  description: ... # A description of the configuration
  icon: ... # Optionally, the name of an icon to use in documentation
  caveats: ... # Optionally, caveats or known issues with this configuration

aspects:
  # The first aspect
  - join-point:
      ... # Join point specification
    advice:
      - ... # Advice specification

  # The second aspect
  - join-point:
      ... # Join point specification
    advice:
      - ... # Advice specification
      - ... # Advice specification
```

## Next

{{<cards>}}
  {{<card
    link="join-points"
    title="Join Points"
    icon="search"
    subtitle="Reference documentation for join point specification"
  >}}
  {{<card
    link="advices"
    title="Advices"
    icon="document-add"
    subtitle="Reference documentation for advice specification"
  >}}
{{</cards>}}
