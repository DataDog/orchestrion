---
title: "Getting Started"
weight: 1

prev: /docs
next: /docs/built-in
---

## Requirements

Orchestrion requires `go 1.21` or later.

> Orchestrion can inject instrumentation which enables use of Datadog's
> <abbr title="Application Security Management">ASM</abbr> features, but those
> are only effectively available on supported platforms (Linux or macOS, on
> AMD64 and ARM64 processor architectures).

## Install Orchestrion

We recommend installing Orchestrion as a project tool dependency, as this
ensures you are in control of the exact versions of Orchestion and the Datadog
tracing library being used; and that your builds are reproductible.

This is achieved using the following steps:

{{% steps %}}

### Step 1
Install Orchestrion in your environment:
```console
$ go install github.com/datadog/orchestrion@{{< releaseTag >}}
```

If necessary, also add the `GOBIN` directory to your `PATH`:
```console
$ export PATH="$PATH:$(go env GOBIN)"
```

### Step 2
Register `orchestrion` in your project's `go.mod` to ensure reproductible builds:
```console
$ orchestrion pin
```

Be sure to check the updated files into source control!

### Step 3

* **Option 1 (Recommended):**

   Use `orchestrion go` instead of just `go`:
   ```console
   $ orchestrion go build .
   $ orchestrion go run .
   $ orchestrion go test ./...
   ```

* **Option 2:**

   Manually specify the `-toolexec` argument to `go` commands:
   ```console
   $ go build -toolexec 'orchestrion toolexec' .
   $ go run -toolexec 'orchestrion toolexec' .
   $ go test -toolexec 'orchestrion toolexec' ./...
   ```

* **Option 3:**

   Add the `-toolexec` argument to the `GOFLAGS` environment variable (_be sure to include the
   quoting as this is required by the `go` toolchain when a flag value includes white space_):
   ```console
   $ export GOFLAGS="${GOFLAGS} '-toolexec=orchestrion toolexec'"
   ```

   Then use `go` commands normally:
   ```console
   $ go build .
   $ go run .
   $ go test ./...
   ```

{{% /steps %}}
