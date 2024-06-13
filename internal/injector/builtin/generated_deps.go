// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.
//
// Code generated by "github.com/datadog/orchestion/internal/injector/builtin/generator -i yaml/*.yml -i yaml/*/*.yml -p builtin -o ./generated.go -d ./generated_deps.go"; DO NOT EDIT.

//go:build tools

package builtin

import (
	_ "fmt"
	_ "github.com/datadog/orchestrion/instrument"
	_ "github.com/datadog/orchestrion/instrument/event"
	_ "github.com/datadog/orchestrion/instrument/net/http"
	_ "gopkg.in/DataDog/dd-trace-go.v1/appsec/events"
	_ "gopkg.in/DataDog/dd-trace-go.v1/contrib/database/sql"
	_ "gopkg.in/DataDog/dd-trace-go.v1/contrib/gin-gonic/gin"
	_ "gopkg.in/DataDog/dd-trace-go.v1/contrib/go-chi/chi"
	_ "gopkg.in/DataDog/dd-trace-go.v1/contrib/go-chi/chi.v5"
	_ "gopkg.in/DataDog/dd-trace-go.v1/contrib/go-redis/redis.v7"
	_ "gopkg.in/DataDog/dd-trace-go.v1/contrib/go-redis/redis.v8"
	_ "gopkg.in/DataDog/dd-trace-go.v1/contrib/gofiber/fiber.v2"
	_ "gopkg.in/DataDog/dd-trace-go.v1/contrib/gomodule/redigo"
	_ "gopkg.in/DataDog/dd-trace-go.v1/contrib/google.golang.org/grpc"
	_ "gopkg.in/DataDog/dd-trace-go.v1/contrib/gorilla/mux"
	_ "gopkg.in/DataDog/dd-trace-go.v1/contrib/gorm.io/gorm.v1"
	_ "gopkg.in/DataDog/dd-trace-go.v1/contrib/jinzhu/gorm"
	_ "gopkg.in/DataDog/dd-trace-go.v1/contrib/labstack/echo.v4"
	_ "gopkg.in/DataDog/dd-trace-go.v1/ddtrace"
	_ "gopkg.in/DataDog/dd-trace-go.v1/ddtrace/ext"
	_ "gopkg.in/DataDog/dd-trace-go.v1/ddtrace/tracer"
	_ "gopkg.in/DataDog/dd-trace-go.v1/internal/appsec"
	_ "gopkg.in/DataDog/dd-trace-go.v1/internal/appsec/emitter/httpsec"
	_ "gopkg.in/DataDog/dd-trace-go.v1/internal/globalconfig"
	_ "gopkg.in/DataDog/dd-trace-go.v1/internal/namingschema"
	_ "math"
	_ "os"
	_ "strconv"
)
