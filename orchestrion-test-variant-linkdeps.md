# Resolve synthetic link dependencies against Go test variants

## Status

Implementation-ready plan for the Orchestrion repository. This document is a
handoff artifact; it does not implement the change. A read-only architecture
review verified the design against the target Orchestrion and Go sources; its
findings about nested resolver deadlock, jobserver self-edges, recursive variant
collection, and environment canonicalization are incorporated below.

## Target repository and baseline

Implement this change in the Orchestrion source checkout. The behavior described
here was investigated against:

- Orchestrion `v1.11.1-0.20260724143832-86453221b0ab`
- Go `1.26.5`
- `golang.org/x/tools v0.48.0`

Before editing, read the target repository's `AGENTS.md` and follow its local
build, test, plan, and version-control requirements. Do not assume that paths or
APIs below are unchanged if implementing against a different revision.

Relevant Orchestrion files at the investigated revision:

- `internal/toolexec/aspect/oncompile-main.go`
- `internal/toolexec/aspect/onlink.go`
- `internal/toolexec/aspect/resolve.go`
- `internal/toolexec/aspect/linkdeps/linkdeps.go`
- `internal/toolexec/proxy/compile.go`
- `internal/jobserver/pkgs/resolve.go`
- `internal/goflags/flags.go`

Relevant Go implementation used as the behavioral reference:

- `$GOROOT/src/cmd/go/internal/load/test.go`, especially
  `recompileForTest`

## Problem statement

Orchestrion records dependencies introduced by instrumentation in a synthetic
`link.deps` archive member. During compilation of `main`, including Go's
generated test main, Orchestrion resolves those packages and injects blank
imports so their archives and initialization functions are linked.

The resolver currently returns ordinary package archives. This is incorrect
when a synthetic dependency directly or transitively imports the package under
test.

For a same-package test, Go compiles an augmented package variant:

```text
example.org/project/model [example.org/project/model.test]
```

Suppose an Orchestrion aspect synthetically links `spans`, and `spans` imports
`model`:

```text
model.test main
├── model [model.test]       fingerprint A
└── synthetic spans
    └── ordinary model      fingerprint B
```

The ordinary `spans` archive embeds fingerprint B as the expected fingerprint
of `model`. The test binary's import configuration selects the augmented
`model` archive with fingerprint A. The linker consequently fails with:

```text
fingerprint mismatch: model has A, import from spans expecting B
```

This is not a stale-cache problem. Rebuilding with `-a` reproduces it. Merely
changing the `packagefile model=...` entry cannot fix it because the expected
fingerprint is embedded in every importing object archive when that archive is
compiled.

## Why Go normally handles this

Go's `recompileForTest` builds a test copy of every package in the test main's
dependency graph that transitively imports the package under test. Given a
visible dependency from the external test package to `spans`, Go produces:

```text
spans [model.test]
└── model [model.test]
```

It similarly rebuilds every intermediate package on a path from `spans` to
`model`.

Orchestrion currently introduces `spans` during `toolexec`, after `cmd/go` has
constructed the test graph and run `recompileForTest`. The fix must make the
synthetic roots visible to a nested test package load, then use the variants Go
builds for that graph.

## Verified solution

A working prototype used `packages.Load`/`go list` with test loading enabled and
an overlay that adds an external test file to the package under test:

```go
package model_test

import (
    _ "example.org/project/spans"
)
```

Loading `model` with this overlay caused Go to emit export archives including:

```text
example.org/project/spans [example.org/project/model.test]
example.org/project/vulnerability [example.org/project/model.test]
```

In the original reproduction, the complete generated closure included
`spans`, `vulnerability`, `iast/crypto/cipher`, and `iast/crypto/hash` test
variants. Substituting that complete closure into the test-main import
configuration, recompiling the test main, linking it, and running the test
succeeded.

This proves that Orchestrion can delegate variant construction and reverse
closure computation to Go instead of implementing a custom package compiler.

## Goals

1. Make Orchestrion-linked same-package tests work when synthetic dependencies
   transitively import the package under test.
2. Reuse Go's `recompileForTest` behavior through `packages.Load` rather than
   duplicating Go's build engine.
3. Select the complete test-variant closure, not only the first synthetic root.
4. Preserve ordinary synthetic link behavior for non-test binaries and for
   roots unrelated to the package under test.
5. Preserve build flags, instrumentation, cache correctness, initialization,
   and bounded nested-resolution behavior.
6. Produce a targeted error if a required variant cannot safely be resolved.

## Non-goals

1. Do not weaken linker fingerprint checking.
2. Do not attempt to place ordinary and test variants of one import path in the
   same binary.
3. Do not rewrite same-package tests into external tests.
4. Do not manually patch fingerprints in Go archives.
5. Do not build a general replacement for `cmd/go`.
6. Do not change the semantics of `links` or `link.deps` for ordinary binaries.
7. Do not silently fall back to ordinary archives after detecting that a test
   variant is required.

## Design overview

Make the existing package-resolution operation test-aware. For each unresolved
synthetic dependency of a generated test main:

1. Derive the package under test from the test executable import path.
2. Resolve the synthetic dependency through the existing `ResolveRequest`, with
   the package under test supplied as optional test-variant context.
3. Determine whether that dependency transitively imports the package under
   test. Return its ordinary closure unchanged when it does not.
4. For an affected dependency, create an in-memory external-test overlay that
   blank-imports it.
5. Load the package under test with `Tests: true` and `NeedForTest`.
6. Collect export archives whose `ForTest` field identifies this package under
   test, and merge them over the ordinary resolution response.
7. Exclude the package-under-test archive so Go's selected archive remains
   authoritative.
8. Use this same resolution path while compiling the generated test main and
   while augmenting the linker's import configuration.
9. Fail with a precise diagnostic if Go cannot construct a required variant.

## Detailed implementation steps

### 1. Add a focused integration fixture first

Create an Orchestrion integration fixture that reproduces the failure before
changing production code. Use neutral package names rather than depending on
dd-iast-go.

Suggested graph:

```text
fixture/model
├── model.go
└── model_test.go          package model

fixture/spans
└── spans.go               imports fixture/model

fixture/hook target
└── aspect injects a linkname whose `links` contains fixture/spans
```

The same-package test only needs one exported test function so that the test
variant has a distinct fingerprint. Ensure the aspect's link dependency is
propagated into the package-under-test archive or otherwise reaches the test
main exactly as in production.

Initial assertion:

```console
go tool orchestrion go test ./path/to/fixture/model
```

must reproduce a fingerprint mismatch before the fix. Avoid asserting literal
fingerprints because they are toolchain-specific.

If the integration test framework cannot retain a deliberately failing fixture,
first add the fixture plus a test harness that expects the current failure, then
flip the expectation in the implementation change.

### 2. Extend the existing resolution request

Add optional test context to `ResolveRequest` rather than introducing another
jobserver subject, request type, response type, or cache:

```go
type ResolveRequest struct {
    // Existing fields omitted.
    Pattern        string
    TestVariantFor string
}
```

`Pattern` remains the package being resolved. A non-empty `TestVariantFor`
asks the resolver to return that package's closure as built for the named
package's tests. Because the field is serialized with the existing request, it
naturally participates in the existing cache key and cannot collide with an
ordinary resolution of the same pattern.

Keep the existing `ResolveRequest.canonicalizeEnviron` implementation and
`envIgnoreList`. In particular, strip volatile `PWD`, `TOOLEXEC_IMPORTPATH`,
and resolve-parent identity values, and normalize `GOTMPDIR` through the
request's `TempDir` field exactly as ordinary resolution already does.
Compile-phase and link-phase requests that differ only in those process-local
values must have the same cache key.

The response remains `ResolveResponse`. Build the ordinary closure first, then
overwrite entries with packages selected by matching `ForTest` before reducing
the graph to the response map. Remove the package-under-test entry from the
result so the archive selected by the outer Go command remains authoritative.

### 3. Detect generated test-main compilation robustly

Use the existing `CompileCommand.TestMain()` helper together with the executable
import path suffix check. At the investigated revision, the established pattern
is:

```go
cmd.TestMain() && strings.HasSuffix(w.ImportPath, ".test")
```

Derive:

```go
packageUnderTest := strings.TrimSuffix(w.ImportPath, ".test")
```

Do not treat every `package main` compilation as a test. Preserve existing
behavior for ordinary commands and other synthetic main packages.

Factor the test-executable detection and package derivation into one helper if
both `OnCompileMain` and `OnLink` need it. Verify what import path is available
during `OnLink`; do not assume without an integration assertion.

### 4. Identify affected synthetic roots

Only a synthetic dependency whose ordinary dependency graph reaches the package
under test needs a test variant. The existing resolver loads the requested
`Pattern` and traverses `packages.Package.Imports` to answer:

```text
Does Pattern directly or transitively import TestVariantFor?
```

Use package identity/import path consistently and protect traversal with a
visited set. If the dependency is unrelated, return the already-collected
ordinary closure without performing the `Tests: true` load. This is important
for internal-package legality: an unrelated cross-module internal dependency
must not be added to the external-test overlay.

### 5. Construct the external-test overlay

Load enough metadata for `packageUnderTest` to obtain its directory and declared
package name. Create overlay contents in memory for a nonexistent absolute path
inside that directory, for example:

```text
<package-dir>/zz_orchestrion_linkdeps_test.go
```

Generate valid Go source using the package's actual name:

```go
package <name>_test

import _ "affected/root"
```

Each resolution handles one `Pattern`, so the overlay contains one import. Use
`go/ast` plus `go/format`, or another repository-standard deterministic
generator, rather than string concatenation that can produce malformed source.

Use `packages.Config.Overlay`; do not write into the user's source tree or
module cache. Confirm that overlays for nonexistent files are supported by the
minimum Go/x-tools version supported by Orchestrion.

The file must be in an external test package. Adding these imports to the
same-package test variant would create an actual test import cycle:

```text
model [model.test] -> spans -> model [model.test]
```

### 6. Load and select Go's test variants

Invoke `packages.Load` for `packageUnderTest` with:

```go
Tests: true
Mode: packages.NeedName |
      packages.NeedCompiledGoFiles |
      packages.NeedImports |
      packages.NeedDeps |
      packages.NeedExportFile |
      packages.NeedForTest
```

Also set:

- `Dir` consistently with existing package resolution;
- the request environment;
- top-level Go build flags from `goflags.Flags`;
- the generated overlay;
- `Logf` using the existing trace logger;
- the request context for cancellation.

Retain the existing rationale for removing or replacing flags such as `-a` and
`-toolexec`, but ensure the nested load still uses the current Orchestrion
binary as its `toolexec`. Preserve all variant-affecting flags, including:

- build tags;
- race/MSAN/ASAN modes;
- coverage and `-coverpkg`;
- PGO;
- compiler experiments;
- platform variables;
- cgo settings.

Check both the top-level `packages.Load` error and every returned
`packages.Package.Errors` entry.

Recursively traverse every returned root's `Imports` graph with a visited set;
do not inspect only the top-level slice returned by `packages.Load`, because
intermediate test copies are generally reachable only through `Imports`.
Collect every traversed package satisfying:

```go
pkg.ForTest == packageUnderTest && pkg.ExportFile != ""
```

Key the response by `pkg.PkgPath`. Exclude synthetic nodes that must not replace
current build entries:

- the package under test itself;
- the generated external test package;
- the `.test` main package.

The package-under-test archive already present in the real test-main import
configuration is authoritative. Do not replace it with the nested load's copy,
even when fingerprints are expected to match.

Validate that every affected root has a selected variant, and that every
variant dependency path needed to reach the package under test is represented.
Treat a missing root export as an error rather than selecting its ordinary
archive.

### 7. Handle nested-resolution recursion and jobserver graph identity

This step is mandatory. The test-aware `packages.Load` itself compiles a nested
test main under Orchestrion. Ensure that this cannot recurse indefinitely,
deadlock in never-build-twice handling, or be rejected as a false import cycle
because the outer and nested test mains share an import path.

First write a focused jobserver/integration test that executes test-aware
resolution from inside an active `OnCompileMain`, not only a standalone
`go list`. Verify the behavior of:

- `TOOLEXEC_IMPORTPATH`;
- the resolve-parent ID propagated by `pkgs.Resolve`;
- jobserver graph `AddEdge` calls;
- never-build-twice build IDs for the two test-main compilations.

A recursion marker is mandatory, not optional. Add a private, clearly named
environment variable to the specialized nested `packages.Load` environment.
When `OnCompileMain` sees this marker on the nested generated test main, it must
skip `OnCompileMain` link-dependency resolution and injection entirely. The
nested test main is a throwaway export artifact and is never linked; the
overlay has already made the affected roots natural dependencies, so its only
purpose is to cause Go to compile the library variants that the outer build
will consume.

Do not suppress ordinary `OnCompile` weaving or archive production for library
packages in the nested graph. Those packages must still be instrumented and
must still carry their `link.deps` metadata.

This full nested-test-main skip is required for two independent reasons:

1. Issuing the same test-variant request recursively uses the same singleflight
   cache key and deadlocks behind the outer request.
2. Falling back to ordinary resolution from the nested test main propagates the
   outer test main as its resolve-parent ID. Because the nested test main has
   the same import path, the jobserver graph observes a false self-edge such as
   `model.test -> model.test` and rejects it.

Keep graph cycle protection enabled globally. If the marker cannot be made
sufficiently local to the nested test-main process, introduce a distinct,
deterministic graph identity for the variant request instead; do not disable
cycle checks.

### 8. Merge variants before compiling the real test main

In `OnCompileMain`, merge the test-aware response before writing the updated
compile importcfg and before compiling the generated synthetic blank-import
source.

Unlike the current ordinary merge, selected test variants must override normal
entries:

```go
for importPath, archive := range variants {
    if importPath == packageUnderTest {
        continue
    }
    reg.PackageFile[importPath] = archive
}
```

Limit overrides to packages explicitly returned for this `ForTest` target. Do
not replace unrelated ordinary dependencies.

Generate blank imports for the original synthetic dependencies as today. The
compiler will then embed the fingerprints of the selected variant archives into
the real test-main object. `LinkDeps` is map-backed and the traversal already
avoids resolved or pending dependencies, so this change does not add a separate
import-list deduplication pass.

### 9. Use the same variants during final linking

Updating only the compile importcfg is insufficient. The real test-main archive
will expect the test-variant fingerprints, so `OnLink` must put those same
variant archives in the linker's importcfg.

Both `OnCompileMain` and `OnLink` call the existing package-resolution helper
with the same optional `TestVariantFor` value. The existing resolver cache makes
an identical second call inexpensive. Correctness requires the same package
fingerprints and effective build configuration, not identical cache file paths:
equivalent archives may legitimately live at different paths. Canonicalize all
variant-affecting inputs so a cache miss still rebuilds fingerprint-equivalent
variants.

Do not persist raw Go-cache archive paths into a long-lived build artifact;
those paths are ephemeral and can become invalid across builds. If testing
shows that compile-time and link-time requests cannot be made semantically
identical, add a stage-local manifest integrated with Orchestrion's artifact
reuse machinery, or persist expected fingerprints rather than assuming path
identity. The normal implementation should prefer one deterministic request
used by both phases.

During link importcfg augmentation:

- preserve the real package-under-test archive already selected by Go;
- override ordinary entries for packages returned by the test-aware response;
- resolve unrelated roots normally;
- keep all other existing entries untouched.

Assert in tests that both the compiler and linker importcfg files select the
variant closure. A passing linker alone is useful but less diagnostic when this
regresses.

### 10. Handle `internal` package restrictions explicitly

The external-test overlay is subject to normal Go `internal` import rules. This
is why affected-root filtering occurs before overlay construction.

Expected common case:

- a cross-module internal synthetic root does not import the user's package
  under test;
- it remains an ordinary root and is not mentioned in the overlay.

If an affected root cannot legally be imported from the external test package,
return a targeted Orchestrion error that names:

- the synthetic root;
- the package under test;
- the transitive dependency relationship;
- the internal visibility failure;
- why using the ordinary archive would produce an invalid test binary.

Do not silently ignore it. A source-level bridge package under the internal
parent could be explored later, but it adds module-cache overlays and additional
visibility constraints and is not required for the first correct fix.

### 11. Add fingerprint diagnostics as defense in depth

Where practical, inspect selected archives before invoking the compiler/linker
and detect an importer expecting a different fingerprint from the archive
selected for the imported path. This may require a small, supported archive
reader because `cmd/internal/goobj` cannot be imported outside the standard
library.

This diagnostic is secondary to variant resolution; do not block the functional
fix on a generalized archive parser unless repository maintainers require it.
At minimum, wrap nested-load failures with enough graph context that users do
not receive only the linker's opaque fingerprint mismatch.

## Test plan

### Unit tests

1. Setting `TestVariantFor` changes the existing resolution request's cache key.
2. Requests differing only in `PWD`, `TOOLEXEC_IMPORTPATH`, or resolve-parent
   identity hash equally, while variant-affecting environment changes do not.
3. Ordinary graph traversal identifies direct and transitive paths to the
   package under test.
4. An unrelated pattern returns its ordinary closure without an overlay load.
5. Overlay generation uses `<actual-package-name>_test` and valid formatted Go.
6. Variant selection prefers `ForTest == packageUnderTest` archives over
   ordinary archives with the same `PkgPath`.
7. The package-under-test archive is not overwritten.
8. Compile/link importcfg merging overrides affected ordinary archives only.
9. Missing affected-root variants return an error.
10. Internal visibility errors produce the targeted diagnostic.
11. The nested-resolution marker skips all `OnCompileMain` work for only the
    nested generated test main.
12. The marker does not suppress normal `OnCompile` weaving or archive
    production for nested library variants.
13. Variant collection recursively visits `Imports` and includes intermediate
    test copies not present in the top-level `packages.Load` result.

### Integration tests

1. **Minimal direct dependency:** synthetic `spans` directly imports the
   same-package-tested `model`; test links and runs.
2. **Transitive closure:** `root -> middle -> model`; both `root` and `middle`
   use `ForTest` variants.
3. **Mixed roots:** one root reaches `model`, one does not; only the first is
   variant-resolved.
4. **Unrelated cross-module internal root:** remains ordinary and does not make
   overlay loading fail.
5. **Affected illegal internal root:** fails with the explicit Orchestrion
   diagnostic, not a linker fingerprint mismatch.
6. **External tests only:** `package model_test` without same-package tests does
   not trigger unnecessary variant resolution.
7. **No tests:** ordinary build/link behavior is unchanged.
8. **Natural dependency already present:** do not create duplicate imports or
   replace a correct existing test variant incorrectly.
9. **Multiple same-package test targets in `./...`:** requests remain isolated
   and cache keys do not cross-contaminate variants.
10. **Nested jobserver execution:** no recursion, deadlock, false cycle, or
    never-build-twice collision.
11. **Cache repeat:** two identical runs reuse resolver/build caches and both
    pass.
12. **Forced rebuild:** `-a` still passes.
13. **Race mode:** run the fixture with `-race` on a supported platform.
14. **Coverage:** run with `-cover` and a representative `-coverpkg` setting.
15. **Build tags:** variant resolution honors a tag that changes the dependency
    graph.
16. **cgo fixture or existing cgo integration:** verify flags and export
    selection if supported by CI.

### Repository-level validation

Use the target repository's documented commands. At minimum, run:

1. Focused unit tests for `internal/jobserver/pkgs` and
   `internal/toolexec/aspect`.
2. The new integration fixture alone.
3. The complete Orchestrion test suite.
4. Static analysis, formatting, and lint commands required by the repository.
5. The original dd-iast-go reproduction if available:

   ```console
   go tool orchestrion go test ./internal/model
   go tool orchestrion go test ./...
   ```

Do not claim success if only the temporary overlay prototype passes; the
jobserver-nested path and final link path must both be exercised.

## Acceptance criteria

1. Same-package tests link when a synthetic dependency transitively imports the
   package under test.
2. Every affected package archive used by the compiler and linker is a variant
   for the correct `ForTest` target.
3. The authoritative package-under-test archive from the real build is
   preserved.
4. Ordinary binaries and unrelated test packages retain existing behavior.
5. Cross-module internal roots unrelated to the test target do not break
   variant resolution.
6. Unsupported affected internal roots fail early with an actionable error.
7. Nested resolution terminates and does not deadlock or trigger a false
   jobserver cycle.
8. Build flags that affect package identity are preserved.
9. Repeated builds are deterministic and cacheable.
10. The original fingerprint mismatch is covered by an automated regression
    test.

## Suggested implementation sequence

Keep changes reviewable in this order, adjusting to the target repository's
commit policy:

1. Add the failing integration fixture and document the expected graph.
2. Add affected-root graph analysis and unit tests.
3. Extend the existing jobserver resolution request with optional test context,
   then add overlay generation and selection tests.
4. Resolve nested jobserver identity/recursion and test it explicitly.
5. Integrate variants into `OnCompileMain`.
6. Integrate the same variants into `OnLink`.
7. Add internal-visibility diagnostics and mixed-root coverage.
8. Run flag, cache, full-suite, and original-reproduction validation.

## Risks and mitigations

### Nested build cost

`packages.Load(Tests: true)` can be expensive. Only invoke it when the requested
synthetic dependency reaches the package under test, and cache the request by
its existing deterministic inputs plus `TestVariantFor`.

### Resolver recursion or deadlock

The nested test load runs under Orchestrion and necessarily compiles another
test main. Put the mandatory private marker in the specialized load's
environment and skip `OnCompileMain` entirely for that nested generated main.
Continue normal weaving for nested library packages. Exercise the real
jobserver path to prove that the marker prevents both same-key singleflight
deadlock and the `model.test -> model.test` false graph self-edge. Do not
disable global cycle protection.

### Variant collision in response maps

Ordinary and test-copy packages share `PkgPath`. Select packages by `ForTest`
before merging them over the ordinary `ResolveResponse`, and omit the package
under test from the final map.

### Flag mismatch

A nested package built under different race, coverage, PGO, tag, experiment, or
platform flags may have a different fingerprint. Reuse the top-level flags and
add representative integration tests.

### Internal visibility

Filter unrelated roots before generating source imports. Fail explicitly for an
affected root that cannot legally be represented.

### Go/x-tools compatibility

`NeedForTest`, overlay behavior, and test package IDs can evolve. Confirm the
minimum supported versions, add build/version-specific handling only if needed,
and test the supported Go matrix.

## Evidence from the original reproduction

The investigated failure was:

```text
fingerprint mismatch: github.com/DataDog/dd-iast-go/internal/model has e57e43bdf6d9880f,
import from github.com/DataDog/dd-iast-go/internal/spans expecting c765c2986c8bb92b
```

The ordinary and augmented model archives had different fingerprints, while
`internal/spans` imported the ordinary model. Orchestrion's generated test-main
source blank-imported `internal/spans` after Go's package graph was fixed.

A nested Orchestrion-enabled test load with an external-test overlay generated
the required `ForTest` archives. Manually selecting the complete closure for
both test-main compilation and linking produced a runnable test binary. This is
the foundation of the proposed design and should be preserved as an integration
regression test rather than replaced with archive fingerprint manipulation.
