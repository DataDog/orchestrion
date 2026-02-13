// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

// Package echoignore provides runtime helpers for Echo integration behavior.
package echoignore

import (
	"reflect"
	"runtime"
	"sync"

	echotrace "github.com/DataDog/dd-trace-go/contrib/labstack/echo.v4/v2"
	"github.com/labstack/echo/v4"
)

var ignoredHandlerNames sync.Map

// New wraps [echo.New] and installs tracing middleware that can honor
// `//orchestrion:ignore` directives on route handlers.
func New() *echo.Echo {
	e := echo.New()
	e.Use(echotrace.Middleware(
		echotrace.WithIgnoreRequest(shouldIgnoreRequest),
	))
	return e
}

func shouldIgnoreRequest(c echo.Context) bool {
	method := c.Request().Method
	path := c.Path()
	if path == "" {
		return false
	}

	for _, route := range c.Echo().Routes() {
		if route.Method == method && route.Path == path && IsIgnoredEchoHandlerName(route.Name) {
			return true
		}
	}

	for _, router := range c.Echo().Routers() {
		for _, route := range router.Routes() {
			if route.Method == method && route.Path == path && IsIgnoredEchoHandlerName(route.Name) {
				return true
			}
		}
	}

	return false
}

// RegisterIgnoredEchoHandlerFunc records a handler function as ignored.
//
// This is intended for generated code and always returns an empty struct so it
// can be safely used in package-level var initializers.
func RegisterIgnoredEchoHandlerFunc(handler any) struct{} {
	fn := reflect.ValueOf(handler)
	if fn.Kind() != reflect.Func {
		return struct{}{}
	}

	meta := runtime.FuncForPC(fn.Pointer())
	if meta == nil {
		return struct{}{}
	}

	name := meta.Name()
	if name == "" {
		return struct{}{}
	}

	ignoredHandlerNames.Store(name, struct{}{})
	// Echo route names are derived from method values and may include "-fm".
	ignoredHandlerNames.Store(name+"-fm", struct{}{})

	return struct{}{}
}

// IsIgnoredEchoHandlerName reports whether the given Echo route handler name is ignored.
func IsIgnoredEchoHandlerName(name string) bool {
	_, found := ignoredHandlerNames.Load(name)
	return found
}
