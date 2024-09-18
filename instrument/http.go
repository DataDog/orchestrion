// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package instrument

import (
	"fmt"
	"net/http"

	httptrace "gopkg.in/DataDog/dd-trace-go.v1/contrib/net/http"
	"gopkg.in/DataDog/dd-trace-go.v1/ddtrace"
	"gopkg.in/DataDog/dd-trace-go.v1/ddtrace/ext"
	"gopkg.in/DataDog/dd-trace-go.v1/ddtrace/tracer"
)

const componentName = "net/http"

func resourceNamer(r *http.Request) string {
	return fmt.Sprintf("%s %s", r.Method, r.URL.Path)
}

func WrapHandler(handler http.Handler) http.Handler {
	return httptrace.WrapHandler(handler, "", "", httptrace.WithResourceNamer(resourceNamer))
	// TODO: We'll reintroduce this later when we stop hard-coding dd-trace-go as above.
	//	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
	//		r = HandleHeader(r)
	//		r = r.WithContext(Report(r.Context(), EventStart, "name", "FooHandler", "verb", r.Method))
	//		defer Report(r.Context(), EventEnd, "name", "FooHandler", "verb", r.Method)
	//		handler.ServeHTTP(w, r)
	//	})
}

func WrapHandlerFunc(handlerFunc http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		httptrace.TraceAndServe(handlerFunc, w, r, &httptrace.ServeConfig{
			Resource: resourceNamer(r),
			SpanOpts: []ddtrace.StartSpanOption{
				tracer.Tag(ext.SpanKind, ext.SpanKindServer),
				tracer.Tag(ext.Component, componentName),
			},
		})
	}
	// TODO: We'll reintroduce this later when we stop hard-coding dd-trace-go as above.
	//	return func(w http.ResponseWriter, r *http.Request) {
	//		r = HandleHeader(r)
	//		r = r.WithContext(Report(r.Context(), EventStart, "name", "FooHandler", "verb", r.Method))
	//		defer Report(r.Context(), EventEnd, "name", "FooHandler", "verb", r.Method)
	//		handlerFunc(w, r)
	//	}
}
