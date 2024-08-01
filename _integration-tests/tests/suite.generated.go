// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.
//
// Code generated by 'go generate'; DO NOT EDIT.

package tests

import (
	chiv5 "orchestrion/integration/tests/chi.v5"
	ddspan "orchestrion/integration/tests/dd-span"
	echov4 "orchestrion/integration/tests/echo.v4"
	fiberv2 "orchestrion/integration/tests/fiber.v2"
	gin "orchestrion/integration/tests/gin"
	goredisv7 "orchestrion/integration/tests/go-redis.v7"
	goredisv8 "orchestrion/integration/tests/go-redis.v8"
	gorm "orchestrion/integration/tests/gorm"
	gormjinzhu "orchestrion/integration/tests/gorm.jinzhu"
	grpc "orchestrion/integration/tests/grpc"
	k8sclientgo "orchestrion/integration/tests/k8s_client_go"
	mongo "orchestrion/integration/tests/mongo"
	mux "orchestrion/integration/tests/mux"
	nethttp "orchestrion/integration/tests/net_http"
	redigo "orchestrion/integration/tests/redigo"
	sql "orchestrion/integration/tests/sql"
	vault "orchestrion/integration/tests/vault"
)

var suite = map[string]testCase{
	"chi.v5":                               new(chiv5.TestCase),
	"dd-span":                              new(ddspan.TestCase),
	"echo.v4":                              new(echov4.TestCase),
	"fiber.v2":                             new(fiberv2.TestCase),
	"gin":                                  new(gin.TestCase),
	"go-redis.v7":                          new(goredisv7.TestCase),
	"go-redis.v8":                          new(goredisv8.TestCase),
	"gorm":                                 new(gorm.TestCase),
	"gorm.jinzhu":                          new(gormjinzhu.TestCase),
	"grpc":                                 new(grpc.TestCase),
	"k8s_client_go/NewCfgFunc":             new(k8sclientgo.TestCaseNewCfgFunc),
	"k8s_client_go/StructLiteralWithParam": new(k8sclientgo.TestCaseStructLiteralWithParam),
	"k8s_client_go/StructLiteralWithoutParam": new(k8sclientgo.TestCaseStructLiteralWithoutParam),
	"mongo":    new(mongo.TestCase),
	"mux":      new(mux.TestCase),
	"net_http": new(nethttp.TestCase),
	"redigo":   new(redigo.TestCase),
	"sql":      new(sql.TestCase),
	"vault":    new(vault.TestCase),
}
