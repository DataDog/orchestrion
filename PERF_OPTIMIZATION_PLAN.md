# Orchestrion Performance Optimization Plan

## Executive Summary

Orchestrion's `-toolexec` approach forks a new process for **every package** in the
dependency graph. For a typical Go project with 200+ packages, this means 200+ process
forks, each paying:

| Overhead per package | Cost | Total for 200 pkgs |
|---|---|---|
| `go env GOMOD` subprocess fork | 10-30ms | **2-6s** |
| 3 NATS loopback round trips | ~1ms | **200ms** |
| Config re-loading (NATS + parse) | ~1ms | **200ms** |
| importcfg parsed twice | ~0.1ms | 20ms |
| Read .go files + byte scan | 1-5ms | **200ms-1s** |
| **Subtotal (no-instrumentation packages)** | **~15-40ms** | **~3-8s of pure waste** |

For packages that DO need instrumentation, add:

| Overhead | Cost |
|---|---|
| Full `go/types` type checking (loads all import archives) | 10-100ms+ |
| DST decoration + AST traversal + restoration per file | 5-20ms |
| Template clone + execute + re-parse per join-point match | 1-5ms each |
| `resolvePackageFiles` for new synthetic imports | 50-500ms (recursive build!) |

**Go's `$GOCACHE` already handles the "no changes" second run** — orchestrion isn't
invoked at all when cached artifacts are valid. The problem is the **first build** and
any cache-busting scenario (`-count=1`, source changes, CI fresh caches).

---

## Current Architecture (How It Works Today)

```
User runs: orchestrion go test ./...
                    |
                    v
    [orchestrion go] starts NATS job server
    pins orchestrion to go.mod
    runs: go test -toolexec="orchestrion toolexec" ./...
                    |
                    v
    Go toolchain compiles N packages, for EACH one:
                    |
                    v
    [fork] orchestrion toolexec compile <args>    (NEW PROCESS)
        |-- connect to NATS server
        |-- NBT StartRequest (NATS round trip)
        |-- parse importcfg (file I/O)
        |-- parse linkdeps from archives (file I/O)
        |-- go env GOMOD (SUBPROCESS FORK!)
        |-- load config via NATS (round trip + parse)
        |-- packageFilterAspects (CPU)
        |-- read all .go files + FileMayMatch scan
        |-- [if aspects match] full type check + AST transform
        |-- [if modified] write new .go files, update importcfg
        |-- run real `go tool compile`
        |-- NBT FinishRequest (NATS round trip)
        |-- [exit process]
```

---

## Optimization Tiers

### Tier 1: Quick Wins (days of work, high ROI)

#### 1.1 Eliminate `go env GOMOD` subprocess fork

**Location:** `internal/toolexec/aspect/oncompile.go:48`
**Problem:** Every toolexec invocation forks `go env GOMOD` as a subprocess. This is
the single largest per-invocation cost (10-30ms) and the result is identical for all
packages in a build.

**Fix:** Set `GOMOD` as an environment variable in `orchestrion go` (the parent
process) before spawning `go build`. The child toolexec processes inherit it.

```go
// In internal/cmd/go.go or internal/goproxy/proxy.go, before exec'ing go:
gomod, _ := goenv.GOMOD(".")
os.Setenv("ORCHESTRION_GOMOD", gomod)
```

Then in `oncompile.go`:
```go
goMod := os.Getenv("ORCHESTRION_GOMOD")
if goMod == "" {
    goMod, err = goenv.GOMOD(".")  // fallback
}
```

**Impact:** Save ~10-30ms per package. For 200 packages = **2-6 seconds saved**.

#### 1.2 Deduplicate importcfg parsing

**Problem:** `importcfg.ParseFile` is called twice per compile invocation — once in
`proxy/compile.go:264` and again in `oncompile.go:43`.

**Fix:** Store the parsed result in `CompileCommand` and reuse it in `OnCompile`.

**Impact:** Minor (~0.1ms per package) but free.

#### 1.3 Cache config loading in the toolexec process

**Problem:** `config.NewLoader(...).Load(ctx)` is called in every `OnCompile`
invocation. While the underlying `packages.Load` is cached in the job server,
each toolexec process still pays a NATS round trip + YAML re-parsing.

**Fix:** Since config is identical across all packages in a build, pass the serialized
config (or a hash pointing to cached config) via an environment variable. The job server
could compute it once and serve it via a dedicated NATS subject with client-side caching
after the first call.

**Impact:** Save ~1ms per package.

---

### Tier 2: Medium Effort, High Impact (1-2 weeks)

#### 2.1 Long-running daemon mode (eliminate process forks)

**Problem:** The fundamental bottleneck is N process forks. Each one pays Go runtime
startup, NATS connection, config loading, and `go env GOMOD`.

**Approach:** Make the toolexec shim ultra-thin. Instead of doing all instrumentation
work in the forked process:

```
[thin shim] orchestrion toolexec compile <args>
    |-- send compile args to job server via single NATS request
    |-- receive back: modified file paths + updated importcfg
    |-- replace args
    |-- exec real compiler
```

All heavy lifting (AST parsing, type checking, transformation) moves into the
long-running job server process, which has:
- Config already loaded and parsed (once)
- GOMOD already known
- No process startup overhead
- Warm caches for type info, parsed files, etc.

The shim could even be a pre-compiled static binary with minimal Go runtime, or use
`os.Exec` directly after a single IPC call.

**Impact:** Reduce per-package overhead from ~15-40ms to ~1-2ms (one IPC round trip).
For 200 packages: **save 3-8 seconds**.

**Complexity:** Medium. The job server already exists and handles concurrent requests.
Main challenge is serializing the file I/O (reading source files, writing modified
files) — the daemon needs access to the same filesystem paths.

#### 2.2 Lazy type checking

**Location:** `internal/injector/injector.go:114`, `internal/injector/check.go:24`

**Problem:** `typeCheck()` runs for ALL packages that pass the file-level heuristic,
even though:
- No production dd-trace-go aspects use `*implements` join points
- Most join points (`functionCall`, `functionName`, `signature`, `structLiteral`) use
  pure AST structural matching — no `types.Info` needed for matching
- `types.Info.Uses` IS needed for the DST decorator's import resolution, but ONLY for
  files that will actually be modified

**Approach — Two-pass matching:**
1. First pass: run matchers WITHOUT type info (using AST-only checks)
2. If any matcher returns "needs type info" or "matched — will modify":
   - Run type checker
   - Re-run the DST decorator with full type info
   - Apply modifications

Most packages will exit at step 1 with zero type checking.

**Impact:** Eliminate type checking for the majority of packages. Type checking loads
every import's archive file and builds full type maps — this can be 10-100ms per package.

#### 2.3 Pre-compiled advice templates

**Location:** `internal/injector/aspect/advice/code/`

**Problem:** Each `Advice.Apply` call:
1. Clones a `text/template`
2. Executes it into a `bytes.Buffer`
3. Parses the output with `go/parser.ParseFile`
4. Converts to DST nodes

For frequently-matched patterns (e.g., HTTP handler wrapping), this happens many times.

**Fix:** Pre-compile common template outputs into cached DST node factories. Use a
lookup keyed by (template name + template arguments hash). Cache the parsed DST subtree
and deep-copy it instead of re-generating from text.

**Impact:** Eliminate template execution + re-parsing overhead per match.

---

### Tier 3: Architectural (weeks of work, transformative)

#### 3.1 Persistent content-addressed instrumentation cache

**Problem:** When Go's build cache is invalidated (`-count=1`, CI fresh cache, any
source change invalidating downstream packages), orchestrion re-instruments everything
from scratch. But most packages' instrumented output hasn't changed — the same aspects
applied to the same source yield the same result.

**Approach:**
```
cache_key = SHA256(source_file_content || aspects_config_hash || orchestrion_version)
cache_dir = ~/.cache/orchestrion/v1/<cache_key_prefix>/<cache_key>
```

Before running `InjectFiles`:
1. Compute cache key for the package (hash of all source files + applicable aspects)
2. Check `~/.cache/orchestrion/` for a hit
3. If hit: copy cached modified files, skip all AST work
4. If miss: run normal pipeline, store result in cache

**Impact:** After the first build, subsequent builds (even with `-count=1` or fresh
`$GOCACHE`) would only re-instrument packages whose source actually changed. For a
200-package project where you changed 1 file, only ~1-5 packages need re-instrumentation.

**Complexity:** Medium. Need careful cache invalidation (aspects hash + file content
hash + orchestrion version). The cache key must include the importcfg (since type
checking depends on dependency versions).

#### 3.2 Hybrid overlay + thin toolexec

**Problem:** `-overlay` alone can't replace `-toolexec` because:
- Can't patch `importcfg` (needed for new synthetic imports)
- Can't embed `link.deps` in archives
- Can't handle `go:linkname` dependencies

**But:** Overlay CAN handle source file replacement, which is the expensive part.

**Approach — split the pipeline:**

```
Phase 1 (pre-build, single process):
    - Load config once
    - Scan all packages, determine which need instrumentation
    - For each: run AST transform, write modified files to cache dir
    - Generate overlay.json mapping original → modified files
    - Compute the set of new synthetic imports needed

Phase 2 (during build, thin toolexec):
    - Source files already replaced via overlay — no AST work needed
    - Toolexec only patches importcfg + handles linkdeps
    - ~1ms per package instead of ~15-40ms
```

```
orchestrion go test ./...
    |
    [Phase 1] orchestrion instruments all packages (single process, cached)
    |         generates overlay.json + synthetic-deps list
    |
    [Phase 2] go test -overlay=overlay.json -toolexec="orchestrion toolexec-lite" ./...
              toolexec-lite: only patches importcfg + linkdeps, no AST work
```

**Impact:** Phase 1 can be incremental (only re-instrument changed files using Tier 3.1
cache). Phase 2 is minimal overhead. Combined: near-instant for unchanged projects.

**Challenge:** Phase 1 needs to know the full dependency graph to determine which
packages need instrumentation. Could use `go list -json ./...` or `packages.Load`.
Also needs to handle the chicken-and-egg problem of synthetic dependencies (a package's
instrumentation might add imports that themselves need instrumentation).

#### 3.3 Background daemon with filesystem watching

**The ultimate goal for development workflow:**

```
orchestrion watch &    # starts background daemon

# Daemon:
# - Watches source files for changes
# - Incrementally re-instruments changed packages
# - Maintains overlay.json always up-to-date
# - Maintains persistent cache

go test ./...          # uses overlay, near-zero orchestrion overhead
```

Developer changes a file → daemon re-instruments that package (~10ms) → next `go test`
picks up the overlay → total added latency: **milliseconds**.

**Impact:** Development-time builds are nearly as fast as uninstrumented builds.

---

## Priority Recommendation

### Phase A: Quick wins (do first, 2-3 days)
1. **1.1** Eliminate `go env GOMOD` → **2-6s saved**
2. **1.2** Deduplicate importcfg parsing → minor
3. **1.3** Cache config loading → ~200ms saved

### Phase B: Daemon mode (next, 1-2 weeks)
4. **2.1** Move instrumentation into job server → **3-8s saved**
5. **2.2** Lazy type checking → **variable, potentially large**

### Phase C: Persistent cache (next, 1-2 weeks)
6. **3.1** Content-addressed cache → **near-instant rebuilds**

### Phase D: Overlay hybrid (ambitious, 2-4 weeks)
7. **3.2** Pre-instrument + overlay → **minimal toolexec overhead**
8. **3.3** Background daemon → **millisecond-level overhead**

---

## Measuring Progress

The existing benchmark suite (`main_test.go:71`) already measures orchestrion overhead
vs baseline across real projects (traefik, delve, gin, etc.). CI runs it 6 times on
dedicated hardware with `benchstat` for statistical rigor.

For per-optimization measurement:
```bash
# Profile a build:
orchestrion --profile-path="$PWD/profiles" --profile=cpu go test ./...
go tool pprof -proto $PWD/profiles/*.pprof > combined.pprof
go tool pprof -http=localhost:6060 combined.pprof

# Or use Datadog APM tracing:
ORCHESTRION_TRACE=true orchestrion go test ./...
```

Key metrics to track:
- **Wall clock**: `time orchestrion go test -count=1 ./...`
- **Per-package overhead**: instrument `OnCompile` with timing
- **Process count**: `ps aux | grep orchestrion | wc -l` during build
- **Cache hit rate**: add metrics to the persistent cache (Tier 3.1)

---

## Benchmark Results (Phase A + C implemented)

Measured on Apple M3 Max, `benchtime=3x`, comparing `main` branch vs optimized branch.

```
goos: darwin
goarch: arm64
cpu: Apple M3 Max

                                                  │  main branch  │       optimized branch        │
                                                  │    sec/op     │   sec/op     vs base          │
/repo=DataDog:orchestrion/variant=baseline-16           13.44           13.40        ~
/repo=DataDog:orchestrion/variant=instrumented-16       18.65           16.55       -11.3%
/repo=jlegrone:tctx/variant=baseline-16                  2.33            2.30        ~
/repo=jlegrone:tctx/variant=instrumented-16             27.04           21.83       -19.3%
/repo=jlegrone:tctx.test/variant=baseline-16             2.75            2.74        ~
/repo=jlegrone:tctx.test/variant=instrumented-16        25.73           22.55       -12.4%
geomean                                                 10.18            9.40        -7.7%
```

### Instrumentation Overhead Reduction

| Benchmark             | Main Overhead | Opt Overhead | Reduction |  % Less |
|-----------------------|---------------|--------------|-----------|---------|
| DataDog:orchestrion   |         5.21s |        3.15s |     2.06s |   39.5% |
| jlegrone:tctx         |        24.71s |       19.53s |     5.18s |   21.0% |
| jlegrone:tctx.test    |        22.98s |       19.81s |     3.17s |   13.8% |

The self-build benchmark (DataDog:orchestrion, ~160 packages) shows **39.5% less
instrumentation overhead**. The external project benchmarks show 13-21% less overhead.

Note: These benchmarks clear `$GOCACHE` each iteration, so the persistent cache (Phase C)
doesn't get to show its warm-cache benefit. The persistent cache would shine in the common
development scenario of `go test -count=1` where Go's cache is busted but instrumented
files haven't changed — in that scenario, the overhead reduction would be much larger.

---

## Appendix: Why `-overlay` Can't Fully Replace `-toolexec`

1. **`importcfg` patching**: When instrumentation adds `import "dd-trace-go/v2/tracer"`,
   that package must appear in the compiler's `importcfg`. Only `-toolexec` can mutate
   this file. (`oncompile.go:126-182`)

2. **`link.deps` in archives**: `go:linkname` dependencies are embedded as custom entries
   in `.a` archives. These can't be expressed as source-level imports (would create
   circular dependencies). (`proxy/compile.go:147-186`)

3. **Build cache invalidation**: `-toolexec` participates in Go's build ID via `-V=full`,
   ensuring caches are busted when aspects change. No equivalent mechanism exists for
   `-overlay`. (`version.go:22-62`)

4. **Package identity**: `TOOLEXEC_IMPORTPATH` tells orchestrion which package is being
   compiled, enabling per-package special-case logic. (`toolexec.go:31`)
