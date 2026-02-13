// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package echoignore

import (
	"net/http"
	"net/http/httptest"
	"reflect"
	"runtime"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/require"
)

func TestRegisterIgnoredEchoHandlerFunc_Function(t *testing.T) {
	RegisterIgnoredEchoHandlerFunc(testFreeHandler)

	name := runtime.FuncForPC(reflect.ValueOf(testFreeHandler).Pointer()).Name()
	require.True(t, IsIgnoredEchoHandlerName(name))
	require.True(t, IsIgnoredEchoHandlerName(name+"-fm"))
}

func TestRegisterIgnoredEchoHandlerFunc_MethodExpression(t *testing.T) {
	RegisterIgnoredEchoHandlerFunc((*testHandler).method)
	RegisterIgnoredEchoHandlerFunc((testHandler).valueMethod)

	ptrMethodExprName := runtime.FuncForPC(reflect.ValueOf((*testHandler).method).Pointer()).Name()
	valueMethodExprName := runtime.FuncForPC(reflect.ValueOf((testHandler).valueMethod).Pointer()).Name()
	require.True(t, IsIgnoredEchoHandlerName(ptrMethodExprName))
	require.True(t, IsIgnoredEchoHandlerName(ptrMethodExprName+"-fm"))
	require.True(t, IsIgnoredEchoHandlerName(valueMethodExprName))
	require.True(t, IsIgnoredEchoHandlerName(valueMethodExprName+"-fm"))
}

func TestRegisterIgnoredEchoHandlerFunc_NonFunction(t *testing.T) {
	RegisterIgnoredEchoHandlerFunc(42)
	require.False(t, IsIgnoredEchoHandlerName("does.not.exist"))
}

func TestShouldIgnoreRequest(t *testing.T) {
	RegisterIgnoredEchoHandlerFunc((*testEchoHandler).readiness)

	e := echo.New()
	h := &testEchoHandler{}
	e.GET("/ready", h.readiness)

	req := httptest.NewRequest(http.MethodGet, "/ready", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	e.Router().Find(req.Method, req.URL.Path, c)

	require.True(t, shouldIgnoreRequest(c))
}

func TestShouldIgnoreRequest_NotIgnored(t *testing.T) {
	e := echo.New()
	h := &testEchoHandler{}
	e.GET("/live", h.liveness)

	req := httptest.NewRequest(http.MethodGet, "/live", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	e.Router().Find(req.Method, req.URL.Path, c)

	require.False(t, shouldIgnoreRequest(c))
}

func testFreeHandler() {}

type testHandler struct{}

func (*testHandler) method() {}

func (testHandler) valueMethod() {}

type testEchoHandler struct{}

func (*testEchoHandler) readiness(echo.Context) error { return nil }

func (*testEchoHandler) liveness(echo.Context) error { return nil }
