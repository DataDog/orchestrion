# Go compiler shadow-taint experiment

## Consolidated fixture suite

Set `TAINT_GO` to the patched toolchain and run the complete behavior matrix with
one command:

```bash
TAINT_GO=/path/to/go-taint-shadow/bin/go go test -count=1 ./experiments/go-shadow/suite
```

`TAINT_GO`'s compiler must self-identify as the patched build: the suite runs
`go tool compile -V=full` first and fails unless the output contains
`iast-taint-shadow-v28`. A stock toolchain emits no shadow labels, so without that
preflight every zero-report fixture would pass and the run would look green while
proving nothing.

Without `TAINT_GO` the fixture test skips and the package still reports `ok`, so a plain
`go test ./...` covers none of these fixtures. `TestFixtureInventory` runs with the stock
toolchain and independently rejects fixture rot — an empty manifest, a case that omits
`dirtyReports`, a case that overrides `TAINT_PATH` through `env`, or a fixture directory
that contributes no cases at all.

The suite covers enabled and disabled compilation, dirty and clean sinks,
static and dynamic calls, interface dispatch, address-taken parameters, stack
growth, channels and selects, maps, closures, GC address reuse, race builds, and
the zero-sized-channel resource regression. Each program under `fixture/`
remains independently runnable for focused debugging.

This experiment is applied to an isolated worktree of the Go source checkout at
`~/dd/golang/go`. It must not be applied to the primary checkout.

Example isolated worktree:

```text
${TMPDIR}/opencode/go-taint-shadow
```

The first patch adds a rollback-safe compiler gate:

```bash
git apply /path/to/orchestrion/experiments/go-shadow/0001-debug-gate.patch
cd src
./make.bash
../bin/go tool compile -d help
```

`-d=taint=1` will gate all later SSA and runtime modifications. With the flag
disabled, the compiler must remain behaviorally identical to the unmodified
toolchain.

## Verified gate

The patched Go 1.26.1 toolchain built successfully with `src/make.bash`. The
fixture in `fixture/` printed `shadow-gate-ok` both with and without
`-gcflags=all=-d=taint=1`, proving the flag is accepted and currently has no
effect when no shadow pass is installed.

## Implemented shadow protocol

The compiler pairs tracked string SSA values with runtime `uint8` labels. Source,
merge, call, and sink decisions execute in the target program; compile-time taint
decisions and sink diagnostics are intentionally excluded.

In addition to per-value SSA labels, the patch keeps a **byte-precise data
shadow**: each existing arena shadow byte is a bitmask carrying one bit per
application byte (byte precision at no extra memory). Because Go string data is
immutable and shared, the data shadow rides all header copies (map/channel/
struct storage, goroutine arguments, sub-slice results such as `strings.Cut`,
`Split`, `TrimPrefix`, `regexp.FindString`) with no per-operation cost, and it
resolves reflective overwrites correctly (a replaced value has its own clean
backing bytes).

The current patch implements:

- `os.Getenv("TAINT_PATH")` source (gated on the key) and a single `os.OpenFile`
  sink (which `os.Open`/`os.Create` funnel through), scanning both the SSA label
  and the value's backing bytes;
- SSA aliases, phis, static calls, function values, interface dispatch, recursion,
  panic/recover, deferred named results, and address-taken parameters;
- authenticated per-goroutine argument/result transitions;
- byte-precise data-shadow propagation through the runtime string/slice
  primitives: `concatstrings*`, `slicebytetostring`, `stringtoslicebyte`,
  `slicerunetostring`, `stringtoslicerune`, `growslice`, `slicecopy`, and
  `makeslicecopy`;
- compiler routing of `copy()` and slice/string `append` through `slicecopy`,
  make+copy through `makeslicecopy`, and `clear()` through a shadow-clear, so
  `bytes.Buffer`/`strings.Builder`/`io.Copy`/encoders (`fmt`, `encoding/json`,
  `encoding/xml`, `database/sql`, `bufio`) carry taint end-to-end;
- byte-precise indexed loads/stores: a byte read from tracked memory carries its
  shadow bit, a byte written to a slice sets or clears the destination bit, and
  overwriting a tainted byte with a clean one removes only that byte's taint;
- closure-environment and non-SSA memory labels;
- dense, atomic arena shadows for heap and stack addresses, including stack moves,
  stack reuse, sweep cleanup, and exact-address heap reuse;
- buffered, unbuffered, closed, and selected `chan string` operations;
- Swiss-map assignment, lookup, overwrite, range, delete, clear, clone, and growth;
- inert compilation when `-d=taint` is disabled.

The durable isolated-toolchain diff is `go-taint-shadow.patch`. It reverse-applies
cleanly to the live experiment worktree.

## Current boundaries

- A byte or rune scalar that crosses a package boundary as a non-string argument
  or return value (e.g. `bytes.Buffer.WriteByte`/`WriteRune`, `lazybuf.append`,
  `utf8.DecodeRuneInString`) does not yet carry its shadow: the interprocedural
  transition protocol carries a single string value, not scalar params/results.
  This is why the byte-at-a-time transforms `path.Clean`/`filepath.Join`,
  `strconv.Quote`, and `net/url.QueryEscape` remain conservatively clean.
- Taint does not propagate through scalar arithmetic, so table-driven transforms
  whose output bytes are computed rather than copied (`base64` encoding) are
  conservatively clean; a value-derived / tainted-index rule is not implemented.
- Arena shadow memory and runtime structure overhead are currently unconditional.
- The implementation is validated on darwin/arm64; 32-bit execution has not been
  exercised.
