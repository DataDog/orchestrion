// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package context

import "strconv"

// WrapGoroutine is woven by orchestrion into every `go` statement it
// compiles, turning `go f(args...)` into `go WrapGoroutine(func() {
// f(args...) })()`. It captures the calling goroutine's context stacks
// synchronously (since WrapGoroutine itself runs in the calling goroutine,
// before the `go` statement spawns anything), and returns a closure that
// installs the propagated stacks on the new goroutine before running body.
//
// This is not meant to be called directly by user code.
//
// Note: body closes over the entire original call expression, not just its
// arguments -- the callee expression `f` and its arguments `args...` are
// both evaluated when body executes (i.e. on the new goroutine) rather than
// before the goroutine is spawned, which differs from plain Go's `go f(x)`
// semantics, where both f and x are evaluated on the calling goroutine
// before the new one ever starts. This matters whenever evaluating f itself
// can panic or has a side effect, e.g. a method value taken off of a
// receiver expression that is itself a function call, or an interface value
// asserted to a function type (the standard library's own time.AfterFunc
// takes this exact shape).
//
// This is an accepted trade-off of rewriting every `go` statement
// generically -- but, contrary to what this comment used to claim, it is
// not a matter of f's static types being unknown: the injector already
// runs full go/types checking before advice is applied (see
// internal/injector/injector.go's typeCheck, exposed to advice through
// context.AdviceContext.ResolveType), so a lifting rewrite along the lines
// of `_f := f; _a0 := args[0]; ...; go WrapGoroutine(func() { _f(_a0, ...)
// })()` would not actually need to spell out any types for the common case.
// The real blockers are narrower: (a) builtins used as the go-statement's
// callee (`close`, `panic`, ...) cannot be assigned to a variable, so they
// cannot be lifted this way; (b) call shapes that forward a single
// multi-value call as every argument (`go f(g())`) need dedicated handling;
// and (c) fixing this at all would require a bespoke Go advice
// implementation rather than the current declarative `wrap-expression`
// template, which only ever substitutes one placeholder for the *whole*
// matched call expression, not `f` and each argument separately.
//
// Two mitigations exist for code that hits this in practice: the closure
// returned below recovers a panic from body and re-panics with additional
// context noting it may stem from evaluating f or args rather than from the
// goroutine's own code (see the recover below, and note it cannot tell the
// two cases apart); and a specific `go` statement can be excluded from this
// rewrite entirely by placing a `//orchestrion:ignore` comment immediately
// above it, restoring exact plain-Go evaluation order for that statement
// (see the package doc comment in doc.go).
func WrapGoroutine(body func()) func() {
	if !enabled() {
		return body
	}

	parent := getBlob()
	entries := registrySnapshot()

	child := make([]any, len(entries))
	for i, e := range entries {
		if e == nil {
			continue
		}
		var p any
		if i < len(parent) {
			p = parent[i]
		}
		child[i] = e.goHook(p)
	}

	return func() {
		setBlob(child)

		defer func() {
			if r := recover(); r != nil {
				panic(&goroutinePanic{cause: r})
			}
		}()
		body()
	}
}

// goroutinePanic wraps a value recovered from a panic raised while running
// the closure [WrapGoroutine] returns, adding context that explains where
// the panic came from without asserting more than is actually known (see
// [WrapGoroutine]'s doc comment: this recover cannot distinguish a panic
// that occurred while evaluating the `go` statement's callee or arguments
// from an ordinary panic raised by the goroutine's own legitimate code,
// since both happen in the exact same call frame).
//
// This stands in for what would otherwise be a single `fmt.Errorf(...,
// "%w", err)` / `fmt.Errorf(..., "%v", r)` call: this package cannot import
// "fmt", because fmt transitively depends on "sync", and "sync" contains a
// real `go` statement of its own (sync.WaitGroup.Go, since Go 1.25) that
// orchestrion's `context.goroutine` aspect would weave -- import-path
// "sync" is not excluded by that aspect, only "runtime" and "internal/**"
// are (see runtime/context/orchestrion.yml). Weaving that statement would
// require the compiled "sync" package to import this package, which -- if
// this package imported "fmt" and therefore "sync" -- would close a real
// import cycle invisible to Go's static build graph, exactly the failure
// mode [registry] and [mutex] already avoid by depending on "sync/atomic"
// (whose own transitive closure is just "unsafe") instead of "sync". See
// also [registry]'s and [mutex]'s doc comments for the general pattern.
type goroutinePanic struct {
	cause any
}

// Error implements the error interface. It never asserts more confidently
// than [WrapGoroutine]'s doc comment allows: the panic may be an ordinary
// one from the goroutine's own code, or may stem from evaluating the `go`
// statement's callee or arguments.
func (p *goroutinePanic) Error() string {
	return "panic on a goroutine spawned via a `go` statement rewritten by orchestrion " +
		"(this may be an ordinary panic from the goroutine's own code, or may stem from " +
		"evaluating the `go` statement's callee or arguments, which happen here rather " +
		"than on the parent goroutine -- see WrapGoroutine's doc comment): " +
		formatGoroutinePanicValue(p.cause)
}

// Unwrap exposes the original panic value when it was an error, so
// errors.Is, errors.As and errors.Unwrap can still reach it -- the
// equivalent of fmt.Errorf's "%w" verb, hand-rolled because this package
// cannot import "fmt" (see [goroutinePanic]). It returns nil (no wrapped
// error) when the original panic value was not itself an error, same as
// fmt.Errorf would if "%v" had been used for it instead.
func (p *goroutinePanic) Unwrap() error {
	err, _ := p.cause.(error)
	return err
}

// formatGoroutinePanicValue renders an arbitrary recovered panic value as
// text, standing in for the "%v" verb fmt.Errorf would otherwise use for a
// non-error panic value (this package cannot import "fmt", see
// [goroutinePanic]). It covers the panic value shapes that matter in
// practice -- errors, strings, fmt.Stringer-shaped values, and the built-in
// numeric/bool kinds -- and falls back to a fixed, type-agnostic message for
// anything else, since rendering an arbitrary struct the way "%v" would
// requires reflection, which this package cannot use for the same reason it
// cannot use "fmt" ("reflect" also transitively depends on "sync").
func formatGoroutinePanicValue(v any) string {
	switch c := v.(type) {
	case error:
		return c.Error()
	case string:
		return c
	case interface{ String() string }:
		return c.String()
	case bool:
		return strconv.FormatBool(c)
	case int:
		return strconv.Itoa(c)
	case int8:
		return strconv.FormatInt(int64(c), 10)
	case int16:
		return strconv.FormatInt(int64(c), 10)
	case int32:
		return strconv.FormatInt(int64(c), 10)
	case int64:
		return strconv.FormatInt(c, 10)
	case uint:
		return strconv.FormatUint(uint64(c), 10)
	case uint8:
		return strconv.FormatUint(uint64(c), 10)
	case uint16:
		return strconv.FormatUint(uint64(c), 10)
	case uint32:
		return strconv.FormatUint(uint64(c), 10)
	case uint64:
		return strconv.FormatUint(c, 10)
	case uintptr:
		return strconv.FormatUint(uint64(c), 10)
	case float32:
		return strconv.FormatFloat(float64(c), 'g', -1, 32)
	case float64:
		return strconv.FormatFloat(c, 'g', -1, 64)
	default:
		return "<non-error panic value>"
	}
}

// Bootstrap dispatches [Hooks.Main] across every registered type and
// installs the resulting stacks on the calling goroutine. It is woven by
// orchestrion into the very start of the program's `func main()`, and is
// not meant to be called directly by user code.
func Bootstrap() {
	if !enabled() {
		return
	}

	entries := registrySnapshot()
	blob := make([]any, len(entries))
	for i, e := range entries {
		if e == nil {
			continue
		}
		blob[i] = e.main()
	}
	setBlob(blob)
}
