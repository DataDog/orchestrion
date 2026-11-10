// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package lib

import (
	"sync"
	"unsafe"
)

type Option struct{ Name string }

// Options is a defined slice type; OptAlias is an alias to the same slice type.
type Options []Option
type OptAlias = []Option

type Box struct{ Name string }

// Woven is a package variable read by the package-initializer aspect's
// template value.
var Woven = "woven"

// Header is the slice header a sink observed at call time.
type Header struct {
	Data unsafe.Pointer
	Len  int
	Cap  int
}

// record collects everything the sinks observe. The woven packages reset it
// before each assertion, so the sink bodies themselves stay trivial.
type record struct {
	mu      sync.Mutex
	calls   [][]string
	headers []Header
	anys    [][]any
	ptrs    [][]*Box
	events  []string
	counts  map[string]int
	wrapped [][]string
}

var Record = &record{}

func (r *record) Reset() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.calls = nil
	r.headers = nil
	r.anys = nil
	r.ptrs = nil
	r.events = nil
	r.counts = nil
	r.wrapped = nil
}

func (r *record) addNames(names []string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.calls = append(r.calls, names)
}

func (r *record) AddOptions(opts []Option) {
	names := make([]string, len(opts))
	for i, opt := range opts {
		names[i] = opt.Name
	}
	r.addNames(names)
	hdr := Header{Data: unsafe.Pointer(unsafe.SliceData(opts)), Len: len(opts), Cap: cap(opts)}
	r.mu.Lock()
	r.headers = append(r.headers, hdr)
	r.mu.Unlock()
}

func (r *record) AddAnys(vals []any) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.anys = append(r.anys, append([]any(nil), vals...))
}

func (r *record) AddPtrs(vals []*Box) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.ptrs = append(r.ptrs, append([]*Box(nil), vals...))
}

func (r *record) AddWrapped(o []Option) {
	names := make([]string, len(o))
	for i, opt := range o {
		names[i] = opt.Name
	}
	r.addNames(names)
}

func (r *record) Event(name string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.events = append(r.events, name)
}

func (r *record) Inc(name string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.counts == nil {
		r.counts = make(map[string]int)
	}
	r.counts[name]++
}

// Names returns the option-name slices recorded by every option-valued sink.
func (r *record) Names() [][]string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([][]string(nil), r.calls...)
}

func (r *record) LastNames() []string {
	all := r.Names()
	if len(all) == 0 {
		return nil
	}
	return all[len(all)-1]
}

func (r *record) Headers() []Header {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]Header(nil), r.headers...)
}

func (r *record) LastHeader() Header {
	all := r.Headers()
	if len(all) == 0 {
		return Header{}
	}
	return all[len(all)-1]
}

func (r *record) Anys() [][]any {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([][]any(nil), r.anys...)
}

func (r *record) Ptrs() [][]*Box {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([][]*Box(nil), r.ptrs...)
}

func (r *record) Events() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]string(nil), r.events...)
}

func (r *record) Count(name string) int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.counts[name]
}

// --- Sinks and values ----------------------------------------------------------

func Sink(opts ...Option)             { Record.AddOptions(opts) }
func Sink3(opts ...Option)            { Record.AddOptions(opts) }
func MutSink(opts ...Option)          { Record.AddOptions(opts) }
func NoopSink(opts ...Option)         { Record.AddOptions(opts) }
func SinkCaptureParam(opts ...Option) { Record.AddOptions(opts) }
func SinkCaptureLocal(opts ...Option) { Record.AddOptions(opts) }
func SinkCalleeWrites(opts ...Option) {
	// The callee assigns to its own variadic slice; the writes must land in
	// the callee's storage, never in the caller's backing array.
	if len(opts) > 0 {
		opts[0] = Named("clobbered")
	}
	Record.AddOptions(opts)
}
func Composed(opts ...Option)             { Record.AddOptions(opts) }
func ComposedEmpty(opts ...Option)        { Record.AddOptions(opts) }
func Redirect(opts ...Option) []Option    { Record.AddOptions(opts); return opts }
func RedirectRev(opts ...Option) []Option { Record.AddOptions(opts); return opts }
func Redirected(opts ...Option) []Option  { Record.AddOptions(opts); return opts }
func WrapSink(opts ...Option) []Option    { Record.AddOptions(opts); return opts }
func WrapSinkRev(opts ...Option) []Option { Record.AddOptions(opts); return opts }

// Wrapped is the wrapper of the wrap-expression combination. It is variadic
// because the append-args advice chained after a wrap-expression operates on
// the wrapper call (ReplaceNode moves the advised node), so the injected
// values land in the wrapper's variadic tail.
func Wrapped(o []Option, rest ...Option) []Option {
	all := make([]Option, 0, len(o)+len(rest))
	all = append(all, o...)
	all = append(all, rest...)
	Record.AddWrapped(all)
	return all
}
func PanicSink(opts ...Option)       { Record.AddOptions(opts) }
func PanicInjSink(opts ...Option)    { Record.AddOptions(opts) }
func PkgInitSink(opts ...Option) int { Record.AddOptions(opts); return len(opts) }

// SinkFixed has a fixed first parameter, for spread calls with fixed
// arguments.
func SinkFixed(_ string, opts ...Option) { Record.AddOptions(opts) }

// PtrSink collects pointer values, for pointer element types.
func PtrSink(vals ...*Box) { Record.AddPtrs(vals) }

// AnySink collects interface values, for the any element type.
func AnySink(vals ...any) { Record.AddAnys(vals) }

// TupleSink is the callee of the rejected sole-tuple call.
func TupleSink(vals ...any) { Record.AddAnys(vals) }

// GenericSink is called through generic inference; it records the values it
// received as dynamic interface values. The T-typed aspect targets callers
// whose type parameter is named T.
func GenericSink[E any](vals ...E) {
	conv := make([]any, len(vals))
	for i, v := range vals {
		conv[i] = v
	}
	Record.AddAnys(conv)
}

// GenericSinkE is the E-named twin of GenericSink, for callers whose element
// type parameter is named E.
func GenericSinkE[E any](vals ...E) {
	GenericSink(vals...)
}

// Server is the receiver type of the woven method-call join point.
type Server struct{ Name string }

func (s Server) Serve(opts ...Option) { Record.AddOptions(opts) }

// NewServer is a receiver factory; its call count is part of the evaluation
// order assertions.
func NewServer() Server { Record.Event("new-server"); return Server{} }

func Named(name string) Option { return Option{Name: name} }
func NewBox(name string) *Box  { return &Box{Name: name} }
func AnyVal() any              { return "anyval" }

func One() Option   { Record.Event("one"); Record.Inc("one"); return Named("one") }
func Two() Option   { Record.Event("two"); Record.Inc("two"); return Named("two") }
func Three() Option { Record.Event("three"); Record.Inc("three"); return Named("three") }

func PanicVal() Option { panic("injected panic") }

// MutateThenTrace mutates the first element of the caller's slice, then returns
// the traced option. The woven clone must snapshot the caller's slice before
// this runs.
func MutateThenTrace(xs []Option) Option {
	if len(xs) > 0 {
		xs[0] = Named("mutated")
	}
	Record.Event("mutate")
	return Named("traced")
}

// TwoOpts returns two options, for the sole-tuple rejection.
func TwoOpts() (Option, Option) { return Named("a"), Named("b") }
