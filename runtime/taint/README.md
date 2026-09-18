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
treated as sources. Direct calls to `os.Open` and `os.OpenFile` are sinks and
emit one JSON report to standard error when the path contains tainted bytes. The
default reporter redacts the value; a custom reporter installed with
`SetReporter` receives it. Set `ORCHESTRION_TAINT_INCLUDE_VALUE=1` to include raw
values in the default JSON reporter for controlled debugging.

## Supported explicit flows

Within instrumented root-module source, the prototype propagates taint through:

- string concatenation and direct byte-index reconstruction;
- named and generic string, byte-slice, and rune-slice types;
- string, byte-slice, and rune-slice conversions;
- two-index slices with explicit lower and upper bounds;
- byte-slice append, copy, clear, and direct single-index assignment forms;
- `strings.Clone`, `Replace`, `ReplaceAll`, `Join`, `Repeat`, `ToUpper`,
  `ToLower`, and `Map`;
- `encoding/base64.Encoding.EncodeToString`, `encoding/json.Marshal`,
  `encoding/xml.Marshal`, `fmt.Sprintf`, `path.Clean`, `filepath.Join`,
  `net/url.QueryEscape`, `regexp.Regexp.ReplaceAllString`, and `strconv.Quote`,
  conservatively;
- `strings.Builder` string and byte-slice writes and snapshots;
- `bytes.Buffer` constructors, byte/string writes, views, snapshots, `Next`,
  `Read`, `ReadBytes`, `ReadString`, scalar `WriteByte`, byte-to-rune
  `WriteRune`, `Truncate`, and `Reset`;
- `bufio.Reader.ReadString` from a tracked `strings.NewReader` source;
- `database/sql.Rows.Scan` from driver `[]byte` into direct `*string` destinations;
- `io.Copy` from a tracked `strings.NewReader` to a concrete `*bytes.Buffer`;
- `bytes.Buffer.WriteTo` into a concrete `*bytes.Buffer`;
- concrete `bytes.Buffer`-to-`bytes.Buffer` `ReadFrom` transfers. Arbitrary
  `io.Reader` sources preserve existing destination taint but are not propagated.
- known fresh-output functions in uninstrumented dependencies, when their
  instrumented root call sites define an explicit summary;
- direct rooted byte and rune scalar transfers through one-argument direct calls,
  inferred local map stores and predeclared-local loads, and inferred buffered
  local channel sends and predeclared-local receives.

Some transforms conservatively taint their complete result instead of computing
exact output positions. When a conservative transform receives multiple source
occurrences, it reports one overlapping full-result range per source ID rather
than collapsing the roots into an unmodelled set.

## Deliberate boundaries

This prototype does not claim universal Go taint tracking. It does not track:

- implicit or control-flow taint;
- scalar transport through function values, multi-result calls, `select`,
  arbitrary expressions, or cross-package code; uninstrumented packages are not
  automatically tracked, so known fresh-output functions require an explicit
  root call-site summary;
- indirect source or sink calls through function values;
- unadapted transforms such as hex encoding;
- unsafe aliases, cgo, assembly, plugins, `go:linkname`, prebuilt code,
  dependencies, or other uninstrumented code.

The address-range registry retains ownership for every tracked storage start,
preserving exact provenance beyond 65,536 entries. That ownership is a strong
reference, so it does more than allow tracked storage to grow: it PINS the
backing array, and the garbage collector cannot reclaim it even after the
application has dropped every reference of its own. A weak-pointer probe over
ten forced `runtime.GC()` cycles observes the array surviving all of them
(`testdata/e2e/case_104_gc_sweep_clears_heap_shadow.go`). Provenance therefore
stays exact — there is no false positive and no false negative — at the cost of
memory that grows with the number of distinct tracked values and is only
released when the process exits, or for pre-rollover generations when
`StartRequest` evicts them. The separate stateful builder and reader registries
remain bounded. If one of those registries saturates, later sinks without exact
ranges emit a report with state `unknown` instead of being treated as clean;
exact ranges continue to take precedence. This conservative fallback is
process-lifetime state. Call `StartRequest` to begin a logical request
generation: new metadata is written to a fresh registry while one prior
generation remains queryable, so escaped values retain precise provenance across
one rollover. A second rollover discards the oldest generation to bound metadata
and permanently enables conservative `unknown` reports for range-free values;
exact ranges in retained generations still take precedence. A process-lifetime
reset remains unsupported.
