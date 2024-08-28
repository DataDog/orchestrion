// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.
//
// Code generated by "github.com/datadog/orchestion/internal/injector/builtin/generator -i=yaml/*.yml -i=yaml/*/*.yml -p=builtin -o=./generated.go -d=./generated_deps.go -C=1 -docs=../../../docs/content/docs/built-in/ -schemadocs=../../../docs/content/contributing/aspects/"; DO NOT EDIT.

//go:build tools

package builtin

import (
	_ "context"
	_ "fmt"
	_ "github.com/datadog/orchestrion/instrument"
	_ "github.com/datadog/orchestrion/instrument/event"
	_ "github.com/datadog/orchestrion/instrument/net/http"
	_ "gopkg.in/DataDog/dd-trace-go.v1/appsec/events"
	_ "gopkg.in/DataDog/dd-trace-go.v1/contrib/IBM/sarama.v1"
	_ "gopkg.in/DataDog/dd-trace-go.v1/contrib/Shopify/sarama"
	_ "gopkg.in/DataDog/dd-trace-go.v1/contrib/aws/aws-sdk-go/aws"
	_ "gopkg.in/DataDog/dd-trace-go.v1/contrib/database/sql"
	_ "gopkg.in/DataDog/dd-trace-go.v1/contrib/gin-gonic/gin"
	_ "gopkg.in/DataDog/dd-trace-go.v1/contrib/go-chi/chi"
	_ "gopkg.in/DataDog/dd-trace-go.v1/contrib/go-chi/chi.v5"
	_ "gopkg.in/DataDog/dd-trace-go.v1/contrib/go-redis/redis.v7"
	_ "gopkg.in/DataDog/dd-trace-go.v1/contrib/go-redis/redis.v8"
	_ "gopkg.in/DataDog/dd-trace-go.v1/contrib/go.mongodb.org/mongo-driver/mongo"
	_ "gopkg.in/DataDog/dd-trace-go.v1/contrib/gofiber/fiber.v2"
	_ "gopkg.in/DataDog/dd-trace-go.v1/contrib/gomodule/redigo"
	_ "gopkg.in/DataDog/dd-trace-go.v1/contrib/google.golang.org/grpc"
	_ "gopkg.in/DataDog/dd-trace-go.v1/contrib/gorm.io/gorm.v1"
	_ "gopkg.in/DataDog/dd-trace-go.v1/contrib/hashicorp/vault"
	_ "gopkg.in/DataDog/dd-trace-go.v1/contrib/internal/httptrace"
	_ "gopkg.in/DataDog/dd-trace-go.v1/contrib/internal/options"
	_ "gopkg.in/DataDog/dd-trace-go.v1/contrib/jinzhu/gorm"
	_ "gopkg.in/DataDog/dd-trace-go.v1/contrib/k8s.io/client-go/kubernetes"
	_ "gopkg.in/DataDog/dd-trace-go.v1/contrib/labstack/echo.v4"
	_ "gopkg.in/DataDog/dd-trace-go.v1/contrib/log/slog"
	_ "gopkg.in/DataDog/dd-trace-go.v1/contrib/net/http"
	_ "gopkg.in/DataDog/dd-trace-go.v1/contrib/redis/go-redis.v9"
	_ "gopkg.in/DataDog/dd-trace-go.v1/ddtrace"
	_ "gopkg.in/DataDog/dd-trace-go.v1/ddtrace/ext"
	_ "gopkg.in/DataDog/dd-trace-go.v1/ddtrace/tracer"
	_ "gopkg.in/DataDog/dd-trace-go.v1/internal"
	_ "gopkg.in/DataDog/dd-trace-go.v1/internal/appsec"
	_ "gopkg.in/DataDog/dd-trace-go.v1/internal/appsec/emitter/httpsec"
	_ "gopkg.in/DataDog/dd-trace-go.v1/internal/globalconfig"
	_ "gopkg.in/DataDog/dd-trace-go.v1/internal/namingschema"
	_ "gopkg.in/DataDog/dd-trace-go.v1/profiler"
	_ "k8s.io/client-go/transport"
	_ "log"
	_ "math"
	_ "net/http"
	_ "os"
	_ "strconv"
)
