---
title: Performance Investigation
type: docs
weight: 99
---

This document provides information on how to obtain performance insight on
Orchestrion, when trying to identify performance optimization opportunities.

## Built-in Profiling

Orchestrion has built-in support for profiling; which can be turned on by
specifying command line arguments to an `orchestrion` command. These flags are
not show by the output of `orchestrion --help` as they are only intended for
deep troubleshooting:

- `-profile-path=DIR` specifies the path to a directory where profiler outputs
  will be written. The directory will be created if it does not exist.
- `-profile=[cpu|heap|trace]` determines which standard Go profilers are enabled
  during the execution. This can be specified multiple times to enable multiple
  profilers.

Note that enabling the profiles may affect the performance of builds (how
exactly depends on the peorifler being used). It may be a good idea to slightly
increase any build timeout that may prevent the build from completing fully.

The main use-case for these involve using the `cpu` or `heap` profilers, then
using the `go tool pprof` tool to combine all generated profiles (a build
involves a great many `orchestrion` processes) into a single one, then
investigating it in some graphical user interface:

```console
$ orchestrion --profile-path="$PWD/profiles" --profile=cpu go build .
$ go tool pprof -proto $PWD/profiles/*.pprof > profile.pprof
$ go tool pprof -http=localhost:6060 profile.pprof
```

The generated profile does not contain _direct_ information about the code being
compiled, and should typically be safe for users to share with maintainers. When
investigating customer performance issues that cannot be reproduced (e.g,
because there is no simple reproduction and their code is private), consider
asking customers to submit both a `cpu` and `heap` profile for investigation.

## `go tool pprof`

For more information on how to use `go tool pprof`, you may refer to the
following resources:

- [The `pprof` documentation][pprof]
- [Profiling Go Programs][go-prof] on the _Go Blog_
- [Profiling Go programs with pprof][jvns] by _Julia Evans_

[pprof]: https://github.com/google/pprof/blob/main/doc/README.md
[go-prof]: https://go.dev/blog/pprof
[jvns]: https://jvns.ca/blog/2017/09/24/profiling-go-with-pprof/
