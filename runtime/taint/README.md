# Experimental Go IAST taint tracking

This package is a research prototype for explicit-flow taint tracking in Go.
It uses Orchestrion to rewrite root-module source while preserving ordinary Go
`string`, `[]byte`, and `[]rune` types.

## Enable the integration

Add the integration to `orchestrion.tool.go`:

```go
//go:build tools

package tools

import _ "github.com/DataDog/orchestrion/runtime/taint/instrument"
```

Build or run the program through Orchestrion. Direct calls to `os.Getenv` are
treated as sources. Direct calls to `os.Open` are sinks and emit one JSON report
to standard error when the path contains tainted bytes. The default reporter
redacts the value; a custom reporter installed with `SetReporter` receives it.
Set `ORCHESTRION_TAINT_INCLUDE_VALUE=1` to include raw values in the default JSON
reporter for controlled debugging.

## Supported explicit flows

Within instrumented root-module source, the prototype propagates taint through:

- string concatenation and direct byte-index reconstruction;
- named and generic string, byte-slice, and rune-slice types;
- string, byte-slice, and rune-slice conversions;
- two-index slices with explicit lower and upper bounds;
- byte-slice append, copy, clear, and direct single-index assignment forms;
- `strings.Clone`, `Replace`, `ReplaceAll`, `Join`, `Repeat`, `ToUpper`,
  `ToLower`, and `Map`;
- `fmt.Sprintf` and `filepath.Join`, conservatively;
- `strings.Builder` string writes and snapshots;
- `bytes.Buffer` constructors, byte/string writes, views, snapshots, `Next`,
  `Truncate`, and `Reset`.

Some transforms conservatively taint their complete result instead of computing
exact output positions.

## Deliberate boundaries

This prototype does not claim universal Go taint tracking. It does not track:

- implicit or control-flow taint;
- arbitrary byte or rune scalars transported through variables, calls, maps, or
  channels;
- indirect source or sink calls through function values;
- unadapted transforms such as URL, base64, hex, or JSON encoding;
- unsafe aliases, cgo, assembly, plugins, `go:linkname`, prebuilt code,
  dependencies, or other uninstrumented code.

The registry retains at most 65,536 occurrences per tracked storage category so
pointer reuse cannot reattach stale metadata without permitting request-driven
unbounded growth. Once a category reaches that limit, new occurrences remain
conservatively clean. A production design still needs request-scoped ownership or
runtime lifecycle integration.
