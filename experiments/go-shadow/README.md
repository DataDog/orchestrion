# Go compiler shadow-taint experiment

## Consolidated fixture suite

Set `TAINT_GO` to the patched toolchain and run the complete behavior matrix with
one command:

```bash
TAINT_GO=/path/to/go-taint-shadow/bin/go go test ./experiments/go-shadow/suite
```

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

The current patch implements:

- `os.Getenv` source labels and `os.Open` runtime sink checks;
- SSA aliases, phis, static calls, function values, interface dispatch, recursion,
  panic/recover, deferred named results, and address-taken parameters;
- authenticated per-goroutine argument/result transitions;
- closure-environment and non-SSA memory labels;
- dense, atomic arena shadows for heap and stack addresses, including stack moves,
  stack reuse, sweep cleanup, and exact-address heap reuse;
- buffered, unbuffered, closed, and selected `chan string` operations;
- Swiss-map assignment, lookup, overwrite, range, delete, clear, clone, and growth;
- inert compilation when `-d=taint` is disabled.

The durable isolated-toolchain diff is `go-taint-shadow.patch`. It reverse-applies
cleanly to the live experiment worktree.

## Current boundaries

- Interprocedural tracking supports exactly one explicit string parameter and one
  string result.
- Globals, foreign memory, and some aggregate/conversion forms remain conservatively
  clean.
- Arena shadow memory and runtime structure overhead are currently unconditional.
- The implementation is validated on darwin/arm64; 32-bit execution has not been
  exercised.
