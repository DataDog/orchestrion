// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

//go:build tools

package tools

import (
	// Tool dependencies
	_ "github.com/google/go-licenses"
	_ "golang.org/x/tools/cmd/stringer"

	// Instrumentation packages
	_ "github.com/datadog/orchestrion/instrument"
	_ "gopkg.in/DataDog/dd-trace-go.v1/contrib/database/sql"
	_ "gopkg.in/DataDog/dd-trace-go.v1/contrib/gin-gonic/gin"
	_ "gopkg.in/DataDog/dd-trace-go.v1/contrib/go-chi/chi.v5"
	_ "gopkg.in/DataDog/dd-trace-go.v1/contrib/gofiber/fiber.v2"
	_ "gopkg.in/DataDog/dd-trace-go.v1/contrib/google.golang.org/grpc"
	_ "gopkg.in/DataDog/dd-trace-go.v1/contrib/gorilla/mux"
	_ "gopkg.in/DataDog/dd-trace-go.v1/contrib/labstack/echo.v4"
	_ "gopkg.in/DataDog/dd-trace-go.v1/ddtrace"
	_ "gopkg.in/DataDog/dd-trace-go.v1/ddtrace/tracer"
)
