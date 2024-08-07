// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.
//
// Code generated by "github.com/datadog/orchestion/internal/injector/builtin/generator -i=yaml/*.yml -i=yaml/*/*.yml -p=builtin -o=./generated.go -d=./generated_deps.go -C=1 -docs=../../../docs/content/docs/built-in/ -schemadocs=../../../docs/content/contributing/aspects/"; DO NOT EDIT.

package builtin

import (
	aspect "github.com/datadog/orchestrion/internal/injector/aspect"
	advice "github.com/datadog/orchestrion/internal/injector/aspect/advice"
	code "github.com/datadog/orchestrion/internal/injector/aspect/advice/code"
	join "github.com/datadog/orchestrion/internal/injector/aspect/join"
)

// Aspects is the list of built-in aspects.
var Aspects = [...]aspect.Aspect{
	// From api/vault.yml
	{
		JoinPoint: join.StructLiteral(join.MustTypeName("github.com/hashicorp/vault/api.Config"), ""),
		Advice: []advice.Advice{
			advice.WrapExpression(code.MustTemplate(
				"{{- .AST.Type -}}{\n  {{- $hasField := false -}}\n  {{ range .AST.Elts }}\n  {{- if eq .Key.Name \"HttpClient\" }}\n  {{- $hasField = true -}}\n  HttpClient: vaulttrace.WrapHTTPClient({{ .Value }}),\n  {{- else -}}\n  {{ . }},\n  {{ end -}}\n  {{ end }}\n  {{- if not $hasField -}}\n  HttpClient: vaulttrace.NewHTTPClient(),\n  {{- end }}\n}",
				map[string]string{
					"vaulttrace": "gopkg.in/DataDog/dd-trace-go.v1/contrib/hashicorp/vault",
				},
			)),
		},
	},
	// From cloud/aws-sdk.yml
	{
		JoinPoint: join.FunctionCall("github.com/aws/aws-sdk-go/aws/session.NewSession"),
		Advice: []advice.Advice{
			advice.WrapExpression(code.MustTemplate(
				"func(sess *session.Session, err error) (*session.Session, error) {\n  if sess != nil {\n    sess = awstrace.WrapSession(sess)\n  }\n  return sess, err\n}({{ . }})",
				map[string]string{
					"awstrace": "gopkg.in/DataDog/dd-trace-go.v1/contrib/aws/aws-sdk-go/aws",
					"session":  "github.com/aws/aws-sdk-go/aws/session",
				},
			)),
		},
	},
	// From databases/go-redis.yml
	{
		JoinPoint: join.OneOf(
			join.FunctionCall("github.com/go-redis/redis/v7.NewClient"),
			join.FunctionCall("github.com/go-redis/redis/v7.NewFailoverClient"),
		),
		Advice: []advice.Advice{
			advice.WrapExpression(code.MustTemplate(
				"func() (client *redis.Client) {\n  client = {{ . }}\n  trace.WrapClient(client)\n  return\n}()",
				map[string]string{
					"redis": "github.com/go-redis/redis/v7",
					"trace": "gopkg.in/DataDog/dd-trace-go.v1/contrib/go-redis/redis.v7",
				},
			)),
		},
	},
	{
		JoinPoint: join.OneOf(
			join.FunctionCall("github.com/go-redis/redis/v8.NewClient"),
			join.FunctionCall("github.com/go-redis/redis/v8.NewFailoverClient"),
		),
		Advice: []advice.Advice{
			advice.WrapExpression(code.MustTemplate(
				"func() (client *redis.Client) {\n  client = {{ . }}\n  trace.WrapClient(client)\n  return\n}()",
				map[string]string{
					"redis": "github.com/go-redis/redis/v8",
					"trace": "gopkg.in/DataDog/dd-trace-go.v1/contrib/go-redis/redis.v8",
				},
			)),
		},
	},
	// From databases/gorm.yml
	{
		JoinPoint: join.FunctionCall("gorm.io/gorm.Open"),
		Advice: []advice.Advice{
			advice.ReplaceFunction("gopkg.in/DataDog/dd-trace-go.v1/contrib/gorm.io/gorm.v1", "Open"),
		},
	},
	{
		JoinPoint: join.FunctionCall("github.com/jinzhu/gorm.Open"),
		Advice: []advice.Advice{
			advice.WrapExpression(code.MustTemplate(
				"func() (*gorm.DB, error) {\n  db, err := {{ . }}\n  if err != nil {\n    return nil, err\n  }\n  return gormtrace.WithCallbacks(db), err\n}()",
				map[string]string{
					"gorm":      "github.com/jinzhu/gorm",
					"gormtrace": "gopkg.in/DataDog/dd-trace-go.v1/contrib/jinzhu/gorm",
				},
			)),
		},
	},
	// From databases/mongo.yml
	{
		JoinPoint: join.FunctionCall("go.mongodb.org/mongo-driver/mongo/options.Client"),
		Advice: []advice.Advice{
			advice.WrapExpression(code.MustTemplate(
				"{{ . }}.SetMonitor(mongotrace.NewMonitor())",
				map[string]string{
					"mongotrace": "gopkg.in/DataDog/dd-trace-go.v1/contrib/go.mongodb.org/mongo-driver/mongo",
					"options":    "go.mongodb.org/mongo-driver/mongo/options",
				},
			)),
		},
	},
	// From databases/redigo.yml
	{
		JoinPoint: join.FunctionCall("github.com/gomodule/redigo/redis.Dial"),
		Advice: []advice.Advice{
			advice.ReplaceFunction("gopkg.in/DataDog/dd-trace-go.v1/contrib/gomodule/redigo", "Dial"),
		},
	},
	{
		JoinPoint: join.FunctionCall("github.com/gomodule/redigo/redis.DialContext"),
		Advice: []advice.Advice{
			advice.ReplaceFunction("gopkg.in/DataDog/dd-trace-go.v1/contrib/gomodule/redigo", "DialContext"),
		},
	},
	{
		JoinPoint: join.FunctionCall("github.com/gomodule/redigo/redis.DialURL"),
		Advice: []advice.Advice{
			advice.ReplaceFunction("gopkg.in/DataDog/dd-trace-go.v1/contrib/gomodule/redigo", "DialURL"),
		},
	},
	// From dd-span.yml
	{
		JoinPoint: join.FunctionBody(join.AllOf(
			join.Directive("dd:span"),
			join.Function(
				join.Receives(join.MustTypeName("context.Context")),
			),
		)),
		Advice: []advice.Advice{
			advice.PrependStmts(code.MustTemplate(
				"{{- $ctx := .FindArgument \"context.Context\" -}}\n{{- $name := .Function.Name -}}\n{{$ctx}} = instrument.Report({{$ctx}}, event.EventStart{{with $name}}, \"function-name\", {{printf \"%q\" .}}{{end}}\n{{- range .DirectiveArgs \"dd:span\" -}}\n  , {{printf \"%q\" .Key}}, {{printf \"%q\" .Value}}\n{{- end -}})\ndefer instrument.Report({{$ctx}}, event.EventEnd{{with $name}}, \"function-name\", {{printf \"%q\" .}}{{end}}\n{{- range .DirectiveArgs \"dd:span\" -}}\n  , {{printf \"%q\" .Key}}, {{printf \"%q\" .Value}}\n{{- end -}})",
				map[string]string{
					"event":      "github.com/datadog/orchestrion/instrument/event",
					"instrument": "github.com/datadog/orchestrion/instrument",
				},
			)),
		},
	},
	{
		JoinPoint: join.FunctionBody(join.AllOf(
			join.Directive("dd:span"),
			join.Function(
				join.Receives(join.MustTypeName("*net/http.Request")),
			),
		)),
		Advice: []advice.Advice{
			advice.PrependStmts(code.MustTemplate(
				"{{- $req := .FindArgument \"*net/http.Request\" -}}\n{{- $name := .Function.Name -}}\n{{$req}} = {{$req}}.WithContext(instrument.Report({{$req}}.Context(), event.EventStart{{with $name}}, \"function-name\", {{printf \"%q\" .}}{{end}}\n{{- range .DirectiveArgs \"dd:span\" -}}\n  , {{printf \"%q\" .Key}}, {{printf \"%q\" .Value}}\n{{- end -}}))\ndefer instrument.Report({{$req}}.Context(), event.EventEnd{{with $name}}, \"function-name\", {{printf \"%q\" .}}{{end}}\n{{- range .DirectiveArgs \"dd:span\" -}}\n  , {{printf \"%q\" .Key}}, {{printf \"%q\" .Value}}\n{{- end -}})",
				map[string]string{
					"event":      "github.com/datadog/orchestrion/instrument/event",
					"instrument": "github.com/datadog/orchestrion/instrument",
				},
			)),
		},
	},
	// From directive/orchestrion-enabled.yml
	{
		JoinPoint: join.AllOf(
			join.Directive("dd:orchestrion-enabled"),
			join.ValueDeclaration(join.MustTypeName("bool")),
		),
		Advice: []advice.Advice{
			advice.AssignValue(code.MustTemplate(
				"true",
				map[string]string{},
			)),
		},
	},
	// From go-main.yml
	{
		JoinPoint: join.AllOf(
			join.PackageName("main"),
			join.FunctionBody(join.Function(
				join.Name("main"),
				join.Signature(
					nil,
					nil,
				),
			)),
		),
		Advice: []advice.Advice{
			advice.PrependStmts(code.MustTemplate(
				"tracer.Start(tracer.WithOrchestrion(map[string]string{\"version\": {{printf \"%q\" Version}}}))\ndefer tracer.Stop()\n\nswitch os.Getenv(\"DD_PROFILING_ENABLED\") {\ncase \"1\", \"true\", \"auto\":\n  // The \"auto\" value can be set if profiling is enabled via the\n  // Datadog Admission Controller. We always turn on the profiler in\n  // the \"auto\" case since we only send profiles after at least a\n  // minute, and we assume anything running that long is worth\n  // profiling.\n  err := profiler.Start(\n    profiler.WithProfileTypes(\n      profiler.CPUProfile,\n      profiler.HeapProfile,\n      // Non-default profiles which are highly likely to be useful:\n      profiler.GoroutineProfile,\n      profiler.MutexProfile,\n    ),\n    profiler.WithTags(\"orchestrion:true\"),\n  )\n  if err != nil {\n    // TODO: is there a better reporting mechanism?\n    // The tracer and profiler already use the stdlib logger, so\n    // we're not adding anything new. But users might be using a\n    // different logger.\n    log.Printf(\"failed to start profiling: %s\", err)\n  }\n  defer profiler.Stop()\n}",
				map[string]string{
					"log":      "log",
					"os":       "os",
					"profiler": "gopkg.in/DataDog/dd-trace-go.v1/profiler",
					"tracer":   "gopkg.in/DataDog/dd-trace-go.v1/ddtrace/tracer",
				},
			)),
		},
	},
	// From grpc.yml
	{
		JoinPoint: join.OneOf(
			join.FunctionCall("google.golang.org/grpc.Dial"),
			join.FunctionCall("google.golang.org/grpc.DialContext"),
			join.FunctionCall("google.golang.org/grpc.NewClient"),
		),
		Advice: []advice.Advice{
			advice.AppendArgs(
				join.MustTypeName("google.golang.org/grpc.DialOption"),
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
				join.MustTypeName("google.golang.org/grpc.ServerOption"),
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
	// From http/chi.yml
	{
		JoinPoint: join.AllOf(
			join.OneOf(
				join.FunctionCall("github.com/go-chi/chi.NewMux"),
				join.FunctionCall("github.com/go-chi/chi.NewRouter"),
			),
			join.Not(join.OneOf(
				join.ImportPath("github.com/go-chi/chi"),
				join.ImportPath("github.com/go-chi/chi/middleware"),
			)),
		),
		Advice: []advice.Advice{
			advice.WrapExpression(code.MustTemplate(
				"func() *chi.Mux {\n  mux := {{ . }}\n  mux.Use(chitrace.Middleware())\n  return mux\n}()",
				map[string]string{
					"chi":      "github.com/go-chi/chi",
					"chitrace": "gopkg.in/DataDog/dd-trace-go.v1/contrib/go-chi/chi",
				},
			)),
		},
	},
	{
		JoinPoint: join.AllOf(
			join.OneOf(
				join.FunctionCall("github.com/go-chi/chi/v5.NewMux"),
				join.FunctionCall("github.com/go-chi/chi/v5.NewRouter"),
			),
			join.Not(join.OneOf(
				join.ImportPath("github.com/go-chi/chi/v5"),
				join.ImportPath("github.com/go-chi/chi/v5/middleware"),
			)),
		),
		Advice: []advice.Advice{
			advice.WrapExpression(code.MustTemplate(
				"func() *chi.Mux {\n  mux := {{ . }}\n  mux.Use(chitrace.Middleware())\n  return mux\n}()",
				map[string]string{
					"chi":      "github.com/go-chi/chi/v5",
					"chitrace": "gopkg.in/DataDog/dd-trace-go.v1/contrib/go-chi/chi.v5",
				},
			)),
		},
	},
	// From http/echo.yml
	{
		JoinPoint: join.FunctionCall("github.com/labstack/echo/v4.New"),
		Advice: []advice.Advice{
			advice.WrapExpression(code.MustTemplate(
				"func() *echo.Echo {\n  e := {{ . }}\n  e.Use(echotrace.Middleware())\n  return e\n}()",
				map[string]string{
					"echo":      "github.com/labstack/echo/v4",
					"echotrace": "gopkg.in/DataDog/dd-trace-go.v1/contrib/labstack/echo.v4",
				},
			)),
		},
	},
	// From http/fiber.yml
	{
		JoinPoint: join.FunctionCall("github.com/gofiber/fiber/v2.New"),
		Advice: []advice.Advice{
			advice.WrapExpression(code.MustTemplate(
				"func() *fiber.App {\n  app := {{ . }}\n  app.Use(fibertrace.Middleware())\n  return app\n}()",
				map[string]string{
					"fiber":      "github.com/gofiber/fiber/v2",
					"fibertrace": "gopkg.in/DataDog/dd-trace-go.v1/contrib/gofiber/fiber.v2",
				},
			)),
		},
	},
	// From http/gin.yml
	{
		JoinPoint: join.OneOf(
			join.FunctionCall("github.com/gin-gonic/gin.Default"),
			join.FunctionCall("github.com/gin-gonic/gin.New"),
		),
		Advice: []advice.Advice{
			advice.WrapExpression(code.MustTemplate(
				"func() *gin.Engine {\n  e := {{ . }}\n  e.Use(gintrace.Middleware(\"\"))\n  return e\n}()",
				map[string]string{
					"gin":      "github.com/gin-gonic/gin",
					"gintrace": "gopkg.in/DataDog/dd-trace-go.v1/contrib/gin-gonic/gin",
				},
			)),
		},
	},
	// From http/gorilla.yml
	{
		JoinPoint: join.StructDefinition(join.MustTypeName("github.com/gorilla/mux.Router")),
		Advice: []advice.Advice{
			advice.InjectDeclarations(code.MustTemplate(
				"type ddRouterConfig struct {\n  ignoreRequest func(*http.Request) bool\n  headerTags    *internal.LockMap\n  resourceNamer func(*Router, *http.Request) string\n  serviceName   string\n  spanOpts      []ddtrace.StartSpanOption\n}\n\nfunc ddDefaultResourceNamer(router *Router, req *http.Request) string {\n  var (\n    match RouteMatch\n    route = \"unknown\"\n  )\n  if router.Match(req, &match) && match.Route != nil {\n    if r, err := match.Route.GetPathTemplate(); err == nil {\n      route = r\n    }\n  }\n  return fmt.Sprintf(\"%s %s\", req.Method, route)\n}\n\nfunc init() {\n  telemetry.LoadIntegration(\"gorilla/mux\")\n  tracer.MarkIntegrationImported(\"github.com/gorilla/mux\")\n}",
				map[string]string{
					"ddtrace":   "gopkg.in/DataDog/dd-trace-go.v1/ddtrace",
					"http":      "net/http",
					"internal":  "gopkg.in/DataDog/dd-trace-go.v1/internal",
					"telemetry": "gopkg.in/DataDog/dd-trace-go.v1/internal/telemetry",
					"tracer":    "gopkg.in/DataDog/dd-trace-go.v1/ddtrace/tracer",
				},
			), []string{}),
			advice.AddStructField("__dd_config", join.MustTypeName("ddRouterConfig")),
		},
	},
	{
		JoinPoint: join.AllOf(
			join.ImportPath("github.com/gorilla/mux"),
			join.FunctionBody(join.Function(
				join.Name("NewRouter"),
			)),
		),
		Advice: []advice.Advice{
			advice.PrependStmts(code.MustTemplate(
				"{{- $res := .Function.Returns 0 -}}\ndefer func() {\n  var analyticsRate float64\n  if internal.BoolEnv(\"DD_TRACE_MUX_ANALYTICS_ENABLED\", false) {\n    analyticsRate = 1.0\n  } else {\n    analyticsRate = globalconfig.AnalyticsRate()\n  }\n\n  {{ $res }}.__dd_config.headerTags = globalconfig.HeaderTagMap()\n  {{ $res }}.__dd_config.ignoreRequest = func(*http.Request) bool { return false }\n  {{ $res }}.__dd_config.resourceNamer = ddDefaultResourceNamer\n  {{ $res }}.__dd_config.serviceName = namingschema.ServiceName(\"mux.router\")\n  {{ $res }}.__dd_config.spanOpts = []ddtrace.StartSpanOption{\n    tracer.Tag(ext.Component, \"gorilla/mux\"),\n    tracer.Tag(ext.SpanKind, ext.SpanKindServer),\n  }\n  if !math.IsNaN(analyticsRate) {\n    {{ $res }}.__dd_config.spanOpts = append(\n      {{ $res }}.__dd_config.spanOpts,\n      tracer.Tag(ext.EventSampleRate, analyticsRate),\n    )\n  }\n}()",
				map[string]string{
					"ddtrace":      "gopkg.in/DataDog/dd-trace-go.v1/ddtrace",
					"ext":          "gopkg.in/DataDog/dd-trace-go.v1/ddtrace/ext",
					"globalconfig": "gopkg.in/DataDog/dd-trace-go.v1/internal/globalconfig",
					"http":         "net/http",
					"internal":     "gopkg.in/DataDog/dd-trace-go.v1/internal",
					"math":         "math",
					"namingschema": "gopkg.in/DataDog/dd-trace-go.v1/internal/namingschema",
					"tracer":       "gopkg.in/DataDog/dd-trace-go.v1/ddtrace/tracer",
				},
			)),
		},
	},
	{
		JoinPoint: join.FunctionBody(join.Function(
			join.Receiver(join.MustTypeName("*github.com/gorilla/mux.Router")),
			join.Name("ServeHTTP"),
		)),
		Advice: []advice.Advice{
			advice.PrependStmts(code.MustTemplate(
				"{{- $r := .Function.Receiver -}}\n{{- $w := .Function.Argument 0 -}}\n{{- $req := .Function.Argument 1 -}}\nif !{{ $r }}.__dd_config.ignoreRequest({{ $req }}) {\n  var (\n    match    RouteMatch\n    route    string\n    spanOpts = options.Copy({{ $r }}.__dd_config.spanOpts...)\n  )\n  if {{ $r }}.Match({{ $req }}, &match) && match.Route != nil {\n    if h, err := match.Route.GetHostTemplate(); err == nil {\n      spanOpts = append(spanOpts, tracer.Tag(\"mux.host\", h))\n    }\n    route, _ = match.Route.GetPathTemplate()\n  }\n  spanOpts = append(spanOpts, httptraceinternal.HeaderTagsFromRequest({{ $req }}, {{ $r }}.__dd_config.headerTags))\n  resource := {{ $r }}.__dd_config.resourceNamer({{ $r }}, {{ $req }})\n\n  // This is a temporary workaround/hack to prevent endless recursion via httptrace.TraceAndServe, which\n  // basically implies passing a shallow copy of this router that ignores all requests down to\n  // httptrace.TraceAndServe.\n  var rCopy Router\n  rCopy = *{{ $r }}\n  rCopy.__dd_config.ignoreRequest = func(*http.Request) bool { return true }\n\n  httptrace.TraceAndServe(&rCopy, {{ $w }}, {{ $req }}, &httptrace.ServeConfig{\n    Service: {{ $r }}.__dd_config.serviceName,\n    Resource: resource,\n    SpanOpts: spanOpts,\n    RouteParams: match.Vars,\n    Route: route,\n  })\n  return\n}",
				map[string]string{
					"http":              "net/http",
					"httptrace":         "gopkg.in/DataDog/dd-trace-go.v1/contrib/net/http",
					"httptraceinternal": "gopkg.in/DataDog/dd-trace-go.v1/contrib/internal/httptrace",
					"options":           "gopkg.in/DataDog/dd-trace-go.v1/contrib/internal/options",
					"tracer":            "gopkg.in/DataDog/dd-trace-go.v1/ddtrace/tracer",
				},
			)),
		},
	},
	// From k8s-client.yml
	{
		JoinPoint: join.StructLiteral(join.MustTypeName("k8s.io/client-go/rest.Config"), ""),
		Advice: []advice.Advice{
			advice.WrapExpression(code.MustTemplate(
				"{{- .AST.Type -}}{\n  {{- $hasField := false -}}\n  {{ range .AST.Elts }}\n  {{- if eq .Key.Name \"WrapTransport\" }}\n  {{- $hasField = true -}}\n  WrapTransport: kubernetestransport.Wrappers({{ .Value }}, kubernetestrace.WrapRoundTripper),\n  {{- else -}}\n  {{ . }},\n  {{ end -}}\n  {{ end }}\n  {{- if not $hasField -}}\n  WrapTransport: kubernetestransport.Wrappers(nil, kubernetestrace.WrapRoundTripper),\n  {{- end }}\n}",
				map[string]string{
					"kubernetestrace":     "gopkg.in/DataDog/dd-trace-go.v1/contrib/k8s.io/client-go/kubernetes",
					"kubernetestransport": "k8s.io/client-go/transport",
				},
			)),
		},
	},
	// From stdlib/database-sql.yml
	{
		JoinPoint: join.FunctionCall("database/sql.Register"),
		Advice: []advice.Advice{
			advice.ReplaceFunction("gopkg.in/DataDog/dd-trace-go.v1/contrib/database/sql", "Register"),
		},
	},
	{
		JoinPoint: join.FunctionCall("database/sql.Open"),
		Advice: []advice.Advice{
			advice.ReplaceFunction("gopkg.in/DataDog/dd-trace-go.v1/contrib/database/sql", "Open"),
		},
	},
	{
		JoinPoint: join.FunctionCall("database/sql.OpenDB"),
		Advice: []advice.Advice{
			advice.ReplaceFunction("gopkg.in/DataDog/dd-trace-go.v1/contrib/database/sql", "OpenDB"),
		},
	},
	// From stdlib/net-http.client.yml
	{
		JoinPoint: join.StructDefinition(join.MustTypeName("net/http.Transport")),
		Advice: []advice.Advice{
			advice.AddStructField("DD__tracer_internal", join.MustTypeName("bool")),
		},
	},
	{
		JoinPoint: join.AllOf(
			join.StructLiteral(join.MustTypeName("net/http.Transport"), ""),
			join.OneOf(
				join.ImportPath("gopkg.in/DataDog/dd-trace-go.v1/ddtrace/tracer"),
				join.ImportPath("gopkg.in/DataDog/dd-trace-go.v1/internal/hostname/httputils"),
				join.ImportPath("gopkg.in/DataDog/dd-trace-go.v1/internal/remoteconfig"),
				join.ImportPath("gopkg.in/DataDog/dd-trace-go.v1/internal/telemetry"),
				join.ImportPath("gopkg.in/DataDog/dd-trace-go.v1/profiler"),
			),
		),
		Advice: []advice.Advice{
			advice.WrapExpression(code.MustTemplate(
				"{{- .AST.Type -}}{\n  DD__tracer_internal: true,\n  {{ range .AST.Elts }}{{ . }},\n  {{ end }}\n}",
				map[string]string{},
			)),
		},
		TracerInternal: true,
	},
	{
		JoinPoint: join.FunctionBody(join.Function(
			join.Name("RoundTrip"),
			join.Receiver(join.MustTypeName("*net/http.Transport")),
		)),
		Advice: []advice.Advice{
			advice.InjectDeclarations(code.MustTemplate(
				"//go:linkname __dd_appsec_RASPEnabled gopkg.in/DataDog/dd-trace-go.v1/internal/appsec.RASPEnabled\nfunc __dd_appsec_RASPEnabled() bool\n\n//go:linkname __dd_httpsec_ProtectRoundTrip gopkg.in/DataDog/dd-trace-go.v1/internal/appsec/emitter/httpsec.ProtectRoundTrip\nfunc __dd_httpsec_ProtectRoundTrip(context.Context, string) error\n\n//go:linkname __dd_tracer_SpanType gopkg.in/DataDog/dd-trace-go.v1/ddtrace/tracer.SpanType\nfunc __dd_tracer_SpanType(string) ddtrace.StartSpanOption\n\n//go:linkname __dd_tracer_ResourceName gopkg.in/DataDog/dd-trace-go.v1/ddtrace/tracer.ResourceName\nfunc __dd_tracer_ResourceName(string) ddtrace.StartSpanOption\n\n//go:linkname __dd_tracer_Tag gopkg.in/DataDog/dd-trace-go.v1/ddtrace/tracer.Tag\nfunc __dd_tracer_Tag(string, any) ddtrace.StartSpanOption\n\n//go:linkname __dd_tracer_StartSpanFromContext gopkg.in/DataDog/dd-trace-go.v1/ddtrace/tracer.StartSpanFromContext\nfunc __dd_tracer_StartSpanFromContext(context.Context, string, ...ddtrace.StartSpanOption) (ddtrace.Span, context.Context)\n\n//go:linkname __dd_tracer_WithError gopkg.in/DataDog/dd-trace-go.v1/ddtrace/tracer.WithError\nfunc __dd_tracer_WithError(error) ddtrace.FinishOption\n\n//go:linkname __dd_tracer_Inject gopkg.in/DataDog/dd-trace-go.v1/ddtrace/tracer.Inject\nfunc __dd_tracer_Inject(ddtrace.SpanContext, any) error\n\ntype __dd_tracer_HTTPHeadersCarrier Header\nfunc (c __dd_tracer_HTTPHeadersCarrier) Set(key, val string) {\n  Header(c).Set(key, val)\n}",
				map[string]string{
					"context": "context",
					"ddtrace": "gopkg.in/DataDog/dd-trace-go.v1/ddtrace",
				},
			), []string{
				"gopkg.in/DataDog/dd-trace-go.v1/ddtrace/tracer",
				"gopkg.in/DataDog/dd-trace-go.v1/internal/appsec",
				"gopkg.in/DataDog/dd-trace-go.v1/internal/appsec/emitter/httpsec",
			}),
			advice.PrependStmts(code.MustTemplate(
				"{{- /* Largely copied from https://github.com/DataDog/dd-trace-go/blob/v1.65.0-rc.2/contrib/net/http/roundtripper.go#L28-L104 */ -}}\n{{- $t := .Function.Receiver -}}\n{{- $req := .Function.Argument 0 -}}\n{{- $res := .Function.Returns 0 -}}\n{{- $err := .Function.Returns 1 -}}\nif !{{ $t }}.DD__tracer_internal {\n  resourceName := fmt.Sprintf(\"%s %s\", {{ $req }}.Method, {{ $req }}.URL.Path)\n  spanName := namingschema.OpName(namingschema.HTTPClient)\n  // Copy the URL so we don't modify the outgoing request\n  url := *{{ $req }}.URL\n  url.User = nil // Don't include userinfo in the http.url tag\n  opts := []ddtrace.StartSpanOption{\n    __dd_tracer_SpanType(ext.SpanTypeHTTP),\n    __dd_tracer_ResourceName(resourceName),\n    __dd_tracer_Tag(ext.HTTPMethod, {{ $req }}.Method),\n    __dd_tracer_Tag(ext.HTTPURL, url.String()),\n    __dd_tracer_Tag(ext.Component, \"net/http\"),\n    __dd_tracer_Tag(ext.SpanKind, ext.SpanKindClient),\n    __dd_tracer_Tag(ext.NetworkDestinationName, url.Hostname()),\n  }\n  if analyticsRate := globalconfig.AnalyticsRate(); !math.IsNaN(analyticsRate) {\n    opts = append(opts, __dd_tracer_Tag(ext.EventSampleRate, analyticsRate))\n  }\n  if port, err := strconv.Atoi(url.Port()); err == nil {\n    opts = append(opts, __dd_tracer_Tag(ext.NetworkDestinationPort, port))\n  }\n  span, ctx := __dd_tracer_StartSpanFromContext({{ $req }}.Context(), spanName, opts...)\n  {{ $req }} = {{ $req }}.Clone(ctx)\n  defer func() {\n    if !events.IsSecurityError({{ $err }}) {\n      span.Finish(__dd_tracer_WithError({{ $err }}))\n    } else {\n      span.Finish()\n    }\n  }()\n\n  if {{ $err }} = __dd_tracer_Inject(span.Context(), __dd_tracer_HTTPHeadersCarrier({{ $req }}.Header)); {{ $err }} != nil {\n    fmt.Fprintf(os.Stderr, \"contrib/net/http.Roundtrip: failed to inject http headers: %v\\n\", {{ $err }})\n  }\n\n  if __dd_appsec_RASPEnabled() {\n    if err := __dd_httpsec_ProtectRoundTrip(ctx, {{ $req }}.URL.String()); err != nil {\n      return nil, err\n    }\n  }\n\n  defer func() {\n    if {{ $err }} != nil {\n      span.SetTag(\"http.errors\", {{ $err }}.Error())\n      span.SetTag(ext.Error, {{ $err }})\n    } else {\n      span.SetTag(ext.HTTPCode, strconv.Itoa({{ $res }}.StatusCode))\n      if {{ $res }}.StatusCode >= 500 && {{ $res}}.StatusCode < 600 {\n        // Treat HTTP 5XX as errors\n        span.SetTag(\"http.errors\", {{ $res }}.Status)\n        span.SetTag(ext.Error, fmt.Errorf(\"%d: %s\", {{ $res }}.StatusCode, StatusText({{ $res }}.StatusCode)))\n      }\n    }\n  }()\n}",
				map[string]string{
					"ddtrace":      "gopkg.in/DataDog/dd-trace-go.v1/ddtrace",
					"events":       "gopkg.in/DataDog/dd-trace-go.v1/appsec/events",
					"ext":          "gopkg.in/DataDog/dd-trace-go.v1/ddtrace/ext",
					"fmt":          "fmt",
					"globalconfig": "gopkg.in/DataDog/dd-trace-go.v1/internal/globalconfig",
					"math":         "math",
					"namingschema": "gopkg.in/DataDog/dd-trace-go.v1/internal/namingschema",
					"os":           "os",
					"strconv":      "strconv",
				},
			)),
		},
	},
	{
		JoinPoint: join.AllOf(
			join.Not(join.ImportPath("net/http")),
			join.OneOf(
				join.FunctionCall("net/http.Get"),
				join.FunctionCall("net/http.Head"),
				join.FunctionCall("net/http.Post"),
				join.FunctionCall("net/http.PostForm"),
			),
		),
		Advice: []advice.Advice{
			advice.WrapExpression(code.MustTemplate(
				"{{- $ctx := .FindArgument \"context.Context\" -}}\n{{- $req := .FindArgument \"*net/http.Request\" }}\n{{- if $ctx -}}\n  instrument.{{ .AST.Fun.Name }}(\n    {{ $ctx }},\n    {{ range .AST.Args }}{{ . }},\n    {{ end }}\n  )\n{{- else if $req -}}\n  instrument.{{ .AST.Fun.Name }}(\n    {{ $req }}.Context(),\n    {{ range .AST.Args }}{{ . }},\n    {{ end }}\n  )\n{{- else -}}\n  {{ . }}\n{{- end -}}",
				map[string]string{
					"instrument": "github.com/datadog/orchestrion/instrument/net/http",
				},
			)),
		},
	},
	// From stdlib/net-http.server.yml
	{
		JoinPoint: join.AllOf(
			join.Configuration(map[string]string{
				"httpmode": "wrap",
			}),
			join.StructLiteral(join.MustTypeName("net/http.Server"), "Handler"),
			join.Not(join.OneOf(
				join.ImportPath("github.com/go-chi/chi/v5"),
				join.ImportPath("github.com/go-chi/chi/v5/middleware"),
				join.ImportPath("golang.org/x/net/http2"),
			)),
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
			join.Not(join.OneOf(
				join.ImportPath("github.com/go-chi/chi/v5"),
				join.ImportPath("github.com/go-chi/chi/v5/middleware"),
				join.ImportPath("golang.org/x/net/http2"),
			)),
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
			join.Not(join.OneOf(
				join.ImportPath("github.com/go-chi/chi/v5"),
				join.ImportPath("github.com/go-chi/chi/v5/middleware"),
				join.ImportPath("golang.org/x/net/http2"),
			)),
		),
		Advice: []advice.Advice{
			advice.PrependStmts(code.MustTemplate(
				"{{- $arg := .Function.Argument 1 -}}\n{{- $name := .Function.Name -}}\n{{$arg}} = {{$arg}}.WithContext(instrument.Report(\n  {{$arg}}.Context(),\n  event.EventStart,\n  {{with $name}}\"function-name\", {{printf \"%q\" .}},{{end}}\n  \"span.kind\", \"server\",\n  \"http.method\", {{$arg}}.Method,\n  \"http.url\", {{$arg}}.URL,\n  \"http.useragent\", {{$arg}}.Header.Get(\"User-Agent\"),\n  {{ range .DirectiveArgs \"dd:span\" -}}{{printf \"%q, %q,\\n\" .Key .Value}}{{ end }}\n))\ndefer instrument.Report(\n  {{$arg}}.Context(),\n  event.EventEnd,\n  {{with $name}}\"function-name\", {{printf \"%q\" .}},{{end}}\n  \"span.kind\", \"server\",\n  \"http.method\", {{$arg}}.Method,\n  \"http.url\", {{$arg}}.URL,\n  \"http.useragent\", {{$arg}}.Header.Get(\"User-Agent\"),\n  {{ range .DirectiveArgs \"dd:span\" -}}{{printf \"%q, %q,\" .Key .Value}}{{- end }}\n)",
				map[string]string{
					"event":      "github.com/datadog/orchestrion/instrument/event",
					"instrument": "github.com/datadog/orchestrion/instrument",
				},
			)),
		},
	},
	// From stdlib/runtime.yml
	{
		JoinPoint: join.StructDefinition(join.MustTypeName("runtime.g")),
		Advice: []advice.Advice{
			advice.AddStructField("__dd_gls", join.MustTypeName("any")),
			advice.AddBlankImport("unsafe"),
			advice.InjectDeclarations(code.MustTemplate(
				"//go:linkname __dd_orchestrion_gls_get __dd_orchestrion_gls_get\nvar __dd_orchestrion_gls_get = func() any {\n  return getg().m.curg.__dd_gls\n}\n\n//go:linkname __dd_orchestrion_gls_set __dd_orchestrion_gls_set\nvar __dd_orchestrion_gls_set = func(val any) {\n  getg().m.curg.__dd_gls = val\n}",
				map[string]string{},
			), []string{}),
		},
	},
	// From stdlib/slog.yml
	{
		JoinPoint: join.FunctionCall("log/slog.New"),
		Advice: []advice.Advice{
			advice.WrapExpression(code.MustTemplate(
				"{{ .AST.Fun }}(slogtrace.WrapHandler({{ index .AST.Args 0 }}))",
				map[string]string{
					"slogtrace": "gopkg.in/DataDog/dd-trace-go.v1/contrib/log/slog",
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
	"gopkg.in/DataDog/dd-trace-go.v1/contrib/jackc/pgx.v5":                             "pgx",
	"gopkg.in/DataDog/dd-trace-go.v1/contrib/labstack/echo.v4":                         "echo",
	"gopkg.in/DataDog/dd-trace-go.v1/contrib/redis/go-redis.v9":                        "redis",
	"gopkg.in/DataDog/dd-trace-go.v1/contrib/segmentio/kafka.go.v0":                    "kafka",
	"gopkg.in/DataDog/dd-trace-go.v1/contrib/valyala/fasthttp.v1":                      "fasthttp",
}

// InjectedPaths is a set of import paths that may be injected by built-in aspects. This list is used to ensure proper
// invalidation of cached artifacts when injected dependencies change.
var InjectedPaths = [...]string{
	"fmt",
	"github.com/datadog/orchestrion/instrument",
	"github.com/datadog/orchestrion/instrument/event",
	"github.com/datadog/orchestrion/instrument/net/http",
	"gopkg.in/DataDog/dd-trace-go.v1/appsec/events",
	"gopkg.in/DataDog/dd-trace-go.v1/contrib/aws/aws-sdk-go/aws",
	"gopkg.in/DataDog/dd-trace-go.v1/contrib/database/sql",
	"gopkg.in/DataDog/dd-trace-go.v1/contrib/gin-gonic/gin",
	"gopkg.in/DataDog/dd-trace-go.v1/contrib/go-chi/chi",
	"gopkg.in/DataDog/dd-trace-go.v1/contrib/go-chi/chi.v5",
	"gopkg.in/DataDog/dd-trace-go.v1/contrib/go-redis/redis.v7",
	"gopkg.in/DataDog/dd-trace-go.v1/contrib/go-redis/redis.v8",
	"gopkg.in/DataDog/dd-trace-go.v1/contrib/go.mongodb.org/mongo-driver/mongo",
	"gopkg.in/DataDog/dd-trace-go.v1/contrib/gofiber/fiber.v2",
	"gopkg.in/DataDog/dd-trace-go.v1/contrib/gomodule/redigo",
	"gopkg.in/DataDog/dd-trace-go.v1/contrib/google.golang.org/grpc",
	"gopkg.in/DataDog/dd-trace-go.v1/contrib/gorm.io/gorm.v1",
	"gopkg.in/DataDog/dd-trace-go.v1/contrib/hashicorp/vault",
	"gopkg.in/DataDog/dd-trace-go.v1/contrib/internal/httptrace",
	"gopkg.in/DataDog/dd-trace-go.v1/contrib/internal/options",
	"gopkg.in/DataDog/dd-trace-go.v1/contrib/jinzhu/gorm",
	"gopkg.in/DataDog/dd-trace-go.v1/contrib/k8s.io/client-go/kubernetes",
	"gopkg.in/DataDog/dd-trace-go.v1/contrib/labstack/echo.v4",
	"gopkg.in/DataDog/dd-trace-go.v1/contrib/log/slog",
	"gopkg.in/DataDog/dd-trace-go.v1/contrib/net/http",
	"gopkg.in/DataDog/dd-trace-go.v1/ddtrace",
	"gopkg.in/DataDog/dd-trace-go.v1/ddtrace/ext",
	"gopkg.in/DataDog/dd-trace-go.v1/ddtrace/tracer",
	"gopkg.in/DataDog/dd-trace-go.v1/internal",
	"gopkg.in/DataDog/dd-trace-go.v1/internal/appsec",
	"gopkg.in/DataDog/dd-trace-go.v1/internal/appsec/emitter/httpsec",
	"gopkg.in/DataDog/dd-trace-go.v1/internal/globalconfig",
	"gopkg.in/DataDog/dd-trace-go.v1/internal/namingschema",
	"gopkg.in/DataDog/dd-trace-go.v1/profiler",
	"k8s.io/client-go/transport",
	"log",
	"math",
	"net/http",
	"os",
	"strconv",
}

// Checksum is a checksum of the built-in configuration which can be used to invalidate caches.
const Checksum = "sha512:4Gi3IVMUORbzxWjHQbXgEgYMSTVz9W/vGmoeH+0ipdwC/ap6CwnYKop/THjVNFxAHS47zLj/m29sAl3dFROSnA=="
