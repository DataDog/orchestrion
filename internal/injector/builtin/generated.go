// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.
//
// Code generated by "github.com/datadog/orchestion/internal/injector/builtin/generator -i yaml/*.yml -p builtin -o ./generated.go"; DO NOT EDIT.

package builtin

import (
	aspect "github.com/datadog/orchestrion/internal/injector/aspect"
	advice "github.com/datadog/orchestrion/internal/injector/aspect/advice"
	code "github.com/datadog/orchestrion/internal/injector/aspect/advice/code"
	join "github.com/datadog/orchestrion/internal/injector/aspect/join"
)

// Aspects is the list of built-in aspects.
var Aspects = [...]aspect.Aspect{
	// From yaml/chi.yml
	{
		JoinPoint: join.AssignmentOf(join.FunctionCall("github.com/go-chi/chi/v5.NewRouter")),
		Advice: []advice.Advice{
			advice.AddComment("//dd:instrumented"),
			advice.AppendStatements(code.MustTemplate(
				"{{.Assignment.LHS}}.Use(chitrace.Middleware())",
				map[string]string{
					"chitrace": "gopkg.in/DataDog/dd-trace-go.v1/contrib/go-chi/chi.v5",
				},
			)),
		},
	},
	// From yaml/database-sql.yml
	{
		JoinPoint: join.FunctionCall("database/sql.Open"),
		Advice: []advice.Advice{
			advice.WrapExpression(code.MustTemplate(
				"sqltrace.Open(\n  {{range .AST.Args}}{{.}},\n{{end}})",
				map[string]string{
					"sqltrace": "gopkg.in/DataDog/dd-trace-go.v1/contrib/database/sql",
				},
			)),
		},
	},
	{
		JoinPoint: join.FunctionCall("database/sql.OpenDB"),
		Advice: []advice.Advice{
			advice.WrapExpression(code.MustTemplate(
				"sqltrace.OpenDB(\n  {{range .AST.Args}}{{.}},\n{{end}})",
				map[string]string{
					"sqltrace": "gopkg.in/DataDog/dd-trace-go.v1/contrib/database/sql",
				},
			)),
		},
	},
	// From yaml/dd-span.yml
	{
		JoinPoint: join.FunctionBody(join.Function(
			join.Directive("dd:span"),
			join.Receives(join.MustTypeName("context.Context")),
		)),
		Advice: []advice.Advice{
			advice.PrependStmts(code.MustTemplate(
				"{{$ctx := .FindArgument \"context.Context\"}}{{$name := .Function.Name}}{{$ctx}} = instrument.Report({{$ctx}}, event.EventStart{{with $name}}, \"function-name\", {{printf \"%q\" .}}{{end}}{{range .DirectiveArgs \"dd:span\"}}, {{printf \"%q\" .Key}}, {{printf \"%q\" .Value}}{{end}})\ndefer instrument.Report({{$ctx}}, event.EventEnd{{with $name}}, \"function-name\", {{printf \"%q\" .}}{{end}}{{range .DirectiveArgs \"dd:span\"}}, {{printf \"%q\" .Key}}, {{printf \"%q\" .Value}}{{end}})",
				map[string]string{
					"event":      "github.com/datadog/orchestrion/instrument/event",
					"instrument": "github.com/datadog/orchestrion/instrument",
				},
			)),
		},
	},
	{
		JoinPoint: join.FunctionBody(join.Function(
			join.Directive("dd:span"),
			join.Receives(join.MustTypeName("*net/http.Request")),
		)),
		Advice: []advice.Advice{
			advice.PrependStmts(code.MustTemplate(
				"{{$req := .FindArgument \"*net/http.Request\"}}{{$name := .Function.Name}}{{$req}} = {{$req}}.WithContext(instrument.Report({{$req}}.Context(), event.EventStart{{with $name}}, \"function-name\", {{printf \"%q\" .}}{{end}}{{range .DirectiveArgs \"dd:span\"}}, {{printf \"%q\" .Key}}, {{printf \"%q\" .Value}}{{end}}))\ndefer instrument.Report({{$req}}.Context(), event.EventEnd{{with $name}}, \"function-name\", {{printf \"%q\" .}}{{end}}{{range .DirectiveArgs \"dd:span\"}}, {{printf \"%q\" .Key}}, {{printf \"%q\" .Value}}{{end}})",
				map[string]string{
					"event":      "github.com/datadog/orchestrion/instrument/event",
					"instrument": "github.com/datadog/orchestrion/instrument",
				},
			)),
		},
	},
	// From yaml/echo.yml
	{
		JoinPoint: join.AssignmentOf(join.FunctionCall("github.com/labstack/echo/v4.New")),
		Advice: []advice.Advice{
			advice.AddComment("/*dd:instrumented*/"),
			advice.AppendStatements(code.MustTemplate(
				"{{.Assignment.LHS}}.Use(echotrace.Middleware())",
				map[string]string{
					"echotrace": "gopkg.in/DataDog/dd-trace-go.v1/contrib/labstack/echo.v4",
				},
			)),
		},
	},
	// From yaml/fiber.yml
	{
		JoinPoint: join.AssignmentOf(join.FunctionCall("github.com/gofiber/fiber/v2.New")),
		Advice: []advice.Advice{
			advice.AddComment("//dd:instrumented"),
			advice.AppendStatements(code.MustTemplate(
				"{{.Assignment.LHS}}.Use(fibertrace.Middleware())",
				map[string]string{
					"fibertrace": "gopkg.in/DataDog/dd-trace-go.v1/contrib/gofiber/fiber.v2",
				},
			)),
		},
	},
	// From yaml/gin.yml
	{
		JoinPoint: join.AssignmentOf(join.OneOf(
			join.FunctionCall("github.com/gin-gonic/gin.Default"),
			join.FunctionCall("github.com/gin-gonic/gin.New"),
		)),
		Advice: []advice.Advice{
			advice.AddComment("//dd:instrumented"),
			advice.AppendStatements(code.MustTemplate(
				"{{.Assignment.LHS}}.Use(gintrace.Middleware(\"\"))",
				map[string]string{
					"gintrace": "gopkg.in/DataDog/dd-trace-go.v1/contrib/gin-gonic/gin",
				},
			)),
		},
	},
	// From yaml/go-main.yml
	{
		JoinPoint: join.AllOf(
			join.PackageName("main"),
			join.FunctionBody(join.Function(
				join.Signature(
					nil,
					nil,
				),
				join.Name("main"),
			)),
		),
		Advice: []advice.Advice{
			advice.PrependStmts(code.MustTemplate(
				"tracer.Start(tracer.WithOrchestrion(map[string]string{\"version\": {{printf \"%q\" Version}}}))\ndefer tracer.Stop()",
				map[string]string{
					"tracer": "gopkg.in/DataDog/dd-trace-go.v1/ddtrace/tracer",
				},
			)),
		},
	},
	// From yaml/gorilla.yml
	{
		JoinPoint: join.FunctionCall("github.com/gorilla/mux.NewRouter"),
		Advice: []advice.Advice{
			advice.WrapExpression(code.MustTemplate(
				"muxtrace.WrapRouter({{.}})",
				map[string]string{
					"muxtrace": "gopkg.in/DataDog/dd-trace-go.v1/contrib/gorilla/mux",
				},
			)),
		},
	},
	// From yaml/grpc.yml
	{
		JoinPoint: join.FunctionCall("google.golang.org/grpc.Dial"),
		Advice: []advice.Advice{
			advice.AppendArgs(
				code.MustTemplate(
					"grpc.WithStreamInterceptor(grpctrace.StreamClientInterceptor())",
					map[string]string{
						"grpc":      "google.golang.org/grpc",
						"grpctrace": "gopkg.in/DataDog/dd-trace-go.v1/contrib/google.golang.org/grpc",
					},
				),
				code.MustTemplate(
					"grpc.WithUnaryInterceptor(grpctrace.UnaryClientInterceptor())",
					map[string]string{
						"grpc":      "google.golang.org/grpc",
						"grpctrace": "gopkg.in/DataDog/dd-trace-go.v1/contrib/google.golang.org/grpc",
					},
				),
			),
		},
	},
	{
		JoinPoint: join.FunctionCall("google.golang.org/grpc.NewServer"),
		Advice: []advice.Advice{
			advice.AppendArgs(
				code.MustTemplate(
					"grpc.StreamInterceptor(grpctrace.StreamServerInterceptor())",
					map[string]string{
						"grpc":      "google.golang.org/grpc",
						"grpctrace": "gopkg.in/DataDog/dd-trace-go.v1/contrib/google.golang.org/grpc",
					},
				),
				code.MustTemplate(
					"grpc.UnaryInterceptor(grpctrace.UnaryServerInterceptor())",
					map[string]string{
						"grpc":      "google.golang.org/grpc",
						"grpctrace": "gopkg.in/DataDog/dd-trace-go.v1/contrib/google.golang.org/grpc",
					},
				),
			),
		},
	},
	// From yaml/net-http.yml
	{
		JoinPoint: join.AllOf(
			join.Configuration(map[string]string{
				"httpmode": "wrap",
			}),
			join.StructLiteral(join.MustTypeName("net/http.Server"), "Handler"),
		),
		Advice: []advice.Advice{
			advice.WrapExpression(code.MustTemplate(
				"//dd:startwrap\ninstrument.WrapHandler({{.}})\n//dd:endwrap",
				map[string]string{
					"instrument": "github.com/datadog/orchestrion/instrument",
				},
			)),
		},
	},
	{
		JoinPoint: join.AllOf(
			join.Configuration(map[string]string{
				"httpmode": "wrap",
			}),
			join.Function(
				join.Name(""),
				join.Signature(
					[]join.TypeName{join.MustTypeName("net/http.ResponseWriter"), join.MustTypeName("*net/http.Request")},
					nil,
				),
			),
		),
		Advice: []advice.Advice{
			advice.WrapExpression(code.MustTemplate(
				"instrument.WrapHandlerFunc({{.}})",
				map[string]string{
					"instrument": "github.com/datadog/orchestrion/instrument",
				},
			)),
		},
	},
	{
		JoinPoint: join.AllOf(
			join.Configuration(map[string]string{
				"httpmode": "report",
			}),
			join.FunctionBody(join.Function(
				join.Signature(
					[]join.TypeName{join.MustTypeName("net/http.ResponseWriter"), join.MustTypeName("*net/http.Request")},
					nil,
				),
			)),
		),
		Advice: []advice.Advice{
			advice.PrependStmts(code.MustTemplate(
				"{{$arg := .Function.Argument 1}}{{$name := .Function.Name}}{{$arg}} = {{$arg}}.WithContext(instrument.Report(\n  {{$arg}}.Context(),\n  event.EventStart,\n  {{with $name}}\"function-name\", {{printf \"%q\" .}},{{end}}\n  \"span.kind\", \"server\",\n  \"http.method\", {{$arg}}.Method,\n  \"http.url\", {{$arg}}.URL,\n  \"http.useragent\", {{$arg}}.Header.Get(\"User-Agent\"),\n  {{range .DirectiveArgs \"dd:span\"}}{{printf \"%q, %q,\\n\" .Key .Value}}{{end}}\n))\ndefer instrument.Report(\n  {{$arg}}.Context(),\n  event.EventEnd,\n  {{with $name}}\"function-name\", {{printf \"%q\" .}},{{end}}\n  \"span.kind\", \"server\",\n  \"http.method\", {{$arg}}.Method,\n  \"http.url\", {{$arg}}.URL,\n  \"http.useragent\", {{$arg}}.Header.Get(\"User-Agent\"),\n  {{range .DirectiveArgs \"dd:span\"}}{{printf \"%q, %q,\" .Key .Value}}{{end}}\n)",
				map[string]string{
					"event":      "github.com/datadog/orchestrion/instrument/event",
					"instrument": "github.com/datadog/orchestrion/instrument",
				},
			)),
		},
	},
}

// RestorerMap is a set of import path to name mappings for packages that would be incorrectly named by restorer.Guess
var RestorerMap = map[string]string{
	"gopkg.in/DataDog/dd-trace-go.v1/contrib/IBM/sarama.v1":                            "sarama",
	"gopkg.in/DataDog/dd-trace-go.v1/contrib/cloud.google.com/go/pubsub.v1":            "pubsub",
	"gopkg.in/DataDog/dd-trace-go.v1/contrib/confluentinc/confluent-kafka-go/kafka.v2": "kafka",
	"gopkg.in/DataDog/dd-trace-go.v1/contrib/dimfeld/httptreemux.v5":                   "httptreemux",
	"gopkg.in/DataDog/dd-trace-go.v1/contrib/elastic/go-elasticsearch.v6":              "elastic",
	"gopkg.in/DataDog/dd-trace-go.v1/contrib/emicklei/go-restful":                      "restful",
	"gopkg.in/DataDog/dd-trace-go.v1/contrib/emicklei/go-restful.v3":                   "restful",
	"gopkg.in/DataDog/dd-trace-go.v1/contrib/go-chi/chi.v5":                            "chi",
	"gopkg.in/DataDog/dd-trace-go.v1/contrib/go-pg/pg.v10":                             "pg",
	"gopkg.in/DataDog/dd-trace-go.v1/contrib/go-redis/redis.v7":                        "redis",
	"gopkg.in/DataDog/dd-trace-go.v1/contrib/go-redis/redis.v8":                        "redis",
	"gopkg.in/DataDog/dd-trace-go.v1/contrib/gofiber/fiber.v2":                         "fiber",
	"gopkg.in/DataDog/dd-trace-go.v1/contrib/google.golang.org/grpc.v12":               "grpc",
	"gopkg.in/DataDog/dd-trace-go.v1/contrib/gopkg.in/jinzhu/gorm.v1":                  "gorm",
	"gopkg.in/DataDog/dd-trace-go.v1/contrib/gorm.io/gorm.v1":                          "gorm",
	"gopkg.in/DataDog/dd-trace-go.v1/contrib/graph-gophers/graphql-go":                 "graphql",
	"gopkg.in/DataDog/dd-trace-go.v1/contrib/labstack/echo.v4":                         "echo",
	"gopkg.in/DataDog/dd-trace-go.v1/contrib/redis/go-redis.v9":                        "redis",
	"gopkg.in/DataDog/dd-trace-go.v1/contrib/segmentio/kafka.go.v0":                    "kafka",
}
