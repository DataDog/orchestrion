---
title: Troubleshooting
weight: 99
prev: /docs/dd-trace-go
---

## Introduction

{{<callout emoji="⚠️">}}
This page provides procedures you can follow to get insight into what happened
during an `orchestrion`-managed build. Some of this information will be
incredibly helpful to our engineers to assist with bug reports. However, please
be mindful that these files often contain details about your project such
as parts of its source code, the names of some of the dependencies your code
uses, etc..., which may not be possible to share publicly over GitHub issues.

If that is your case, or if you have a doubt whether you can share those files
publicly or not, please enagage with Datadog support in order to share some or
all of this information privately instead.
{{</callout>}}

## Preserving the work tree

Orchestrion records data that can allow re-constructing all transformations that
it applied to your source files within the `go build` work tree. By default, the
`go` toolchain cleans up this directory at the end of a build, but passing the
`-work` flag will cause it to preserve those directories for later inspection.

```console
$ orchestrion go build -work ./...
WORK=/tmp/go-build2455442813
```

The `go` toolchain working directory is printed out at the very beginning of the
build, preceded by the `WORK=` marker. This directory contains one or more
sub-directories, each corresponding to one of the built `go` packages.

## Contents of the work tree

When `orchestrion` injects code into a source file, it writes the complete
modified file in the relevant package's stage directory, int the
`orchestrion/src` sub-directory. If it modifies package import configurations,
the original configuration file is preserved with a `.original` suffix.

For example, this could look something like the following:

{{<filetree/container>}}
  {{<filetree/folder name="WORK">}}
    {{<filetree/folder name="b001">}}
      {{<filetree/folder name="exe">}}
        {{<filetree/file name="a.out">}}
      {{</filetree/folder>}}
      {{<filetree/file name="_pkg_.a">}}
      {{<filetree/file name="importcfg">}}
      {{<filetree/file name="importcfg.link">}}
      {{<filetree/file name="importcfg.link.original">}}
    {{</filetree/folder>}}
    {{<filetree/folder name="b002">}}
      {{<filetree/folder name="orchestrion">}}
        {{<filetree/folder name="src">}}
          {{<filetree/file name="server.go">}}
        {{</filetree/folder>}}
      {{</filetree/folder>}}
      {{<filetree/file name="_pkg_.a">}}
      {{<filetree/file name="importcfg">}}
      {{<filetree/file name="importcfg.original">}}
    {{</filetree/folder>}}
    {{<filetree/folder name="b003" state="closed">}}{{</filetree/folder>}}
    {{<filetree/folder name="..." state="closed">}}{{</filetree/folder>}}
  {{</filetree/folder>}}
{{</filetree/container>}}

The contents of these files is human-readable, and you may inspect them to get
an understanding of what happened, see if Orchestrion did anything unexpected.
If you find yourself unable to make sense of what you see in these files, our
engineers will be happy to assist.

## Extensive Logging

### Configuring Log Level

Orchestrion can be configured to output extensive logging during operations.
This is configured by setting the `ORCHESTRION_LOG_LEVEL` environment variable
or `--log-level` orchestrion flag to one of the following values:

`ORCHESTRION_LOG_LEVEL` | Description
------------------------|-------------------------------------------------------
`NONE`, `OFF` (default) | No logging output is produced
`ERROR`                 | Logs only error information
`WARN`                  | Logs error information and warnings
`INFO`                  | Logs error information, warnings, and informational messages
`DEBUG`                 | Detailed logging
`TRACE`                 | Extremely detailed logging

{{<callout emoji="⚠️">}}
Setting `ORCHESTRION_LOG_LEVEL` to the `DEBUG` or `TRACE` levels may have a
significant impact on build performance, and we do not generally recommend using
these settings during normal operations.
{{</callout>}}

### Sending log output to files

By default, logging messages are sent to the process' console. It can however be
useful to instead send those messages to one or more files, as these can be
easier to investigate after the fact. To do so, set the `ORCHESTRION_LOG_FILE`
environment variable or `--log-file` orchestrion flag to the path of the file to
write logs to.

{{<callout type="info">}}
Setting `ORCHESTRION_LOG_FILE` changes the default value of
`ORCHESTRION_LOG_LEVEL` to `WARN`.
{{</callout>}}

The tokens `$PID` and `${PID}` in `ORCHESTRION_LOG_FILE` are automatically
replaced by the logging process' PID. This can cause a significant amount of
files to be created when building large projects, but reduces contention writing
to each file.

If the file already exists, new entries are appended to it, instead of
clobbering it.
