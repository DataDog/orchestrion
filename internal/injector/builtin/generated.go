// Code generated by "-i yaml/*.yml -p builtin -o ./generated.go"; DO NOT EDIT.

package builtin

import (
	injector "github.com/datadog/orchestrion/internal/injector"
	advice "github.com/datadog/orchestrion/internal/injector/advice"
	code "github.com/datadog/orchestrion/internal/injector/advice/code"
	join "github.com/datadog/orchestrion/internal/injector/join"
)

var Aspects = [...]injector.Aspect{
	// From yaml/chi.yml
	{
		JoinPoint: join.AssignmentOf(join.FunctionCall("github.com/go-chi/chi/v5.NewRouter")),
		Advice: []advice.Advice{
			advice.AddComment("//dd:instrumented"),
			advice.AppendStatements(code.MustTemplate(
				"{{.Assignment.LHS}}.Use(chitrace.ChiV5Middleware())",
				map[string]string{
					"chitrace": "gopkg.in/DataDog/dd-trace-go.v1/contrib/go-chi/chi.v5",
				},
			)),
		},
	},
	// From yaml/database-sql.yml
	{
		JoinPoint: join.FunctionCall("sql.Open"),
		Advice: []advice.Advice{
			advice.WrapExpression(code.MustTemplate(
				"sqltrace.Open({{.FunctionCall.Arguments}})",
				map[string]string{
					"sqltrace": "gopkg.in/DataDog/dd-trace-go.v1/contrib/database/sql",
				},
			)),
		},
	},
	{
		JoinPoint: join.FunctionCall("sql.OpenDB"),
		Advice: []advice.Advice{
			advice.WrapExpression(code.MustTemplate(
				"sqltrace.OpenDB({{.FunctionCall.Arguments}})",
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
			join.OneOfFunctions(
				join.Receives(join.MustTypeName("context.Context")),
				join.Receives(join.MustTypeName("*net/http.Request")),
			),
		)),
		Advice: []advice.Advice{
			advice.PrependStmts(code.MustTemplate(
				"{{$ctx := or (.FindArgument \"context.Context\") (printf \"%s.Context()\" (.FindArgument \"*net/http.Request\"))}}{{$name := .Function.Name}}instrument.Report({{$ctx}}, event.EventStart{{with $name}}, \"name\", {{printf \"%q\" .}}{{end}}{{range .DirectiveArgs \"dd:span\"}}, {{printf \"%q\" .Key}}, {{printf \"%q\" .Value}}{{end}})\ndefer instrument.Report({{$ctx}}, event.EventEnd{{with $name}}, \"name\", {{printf \"%q\" .}}{{end}}{{range .DirectiveArgs \"dd:span\"}}, {{printf \"%q\" .Key}}, {{printf \"%q\" .Value}}{{end}})",
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
				"{{.Assignment.LHS}} = {{.Assignment.LHS}}.Use(echotrace.EchoV4Middleware())",
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
				"{{.Assignment.LHS}} = {{.Assignment.LHS}}.Use(fibertrace.FiberV2Middleware())",
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
				"{{.Assignment.LHS}} = {{.Assignment.LHS}}.Use(gintrace.Middleware(\"\"))",
				map[string]string{
					"gintrace": "gopkg.in/DataDog/dd-trace-go.v1/contrib/gin-gonic/gin",
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
					"grpc.WithStreamInterceptor(grpctrace.GRPCStreamClientInterceptor())",
					map[string]string{
						"grpc":      "google.golang.org/grpc",
						"grpctrace": "gopkg.in/DataDog/dd-trace-go.v1/contrib/google.golang.org/grpc",
					},
				),
				code.MustTemplate(
					"grpc.WithUnaryInterceptor(grpctrace.GRPCUnaryClientInterceptor())",
					map[string]string{
						"grpctrace": "gopkg.in/DataDog/dd-trace-go.v1/contrib/google.golang.org/grpc",
						"grpc":      "google.golang.org/grpc",
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
					"grpc.StreamInterceptor(grpctrace.GRPCStreamServerInterceptor())",
					map[string]string{
						"grpc":      "google.golang.org/grpc",
						"grpctrace": "gopkg.in/DataDog/dd-trace-go.v1/contrib/google.golang.org/grpc",
					},
				),
				code.MustTemplate(
					"grpc.UnaryInterceptor(grpctrace.GRPCUnaryServerInterceptor())",
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
		JoinPoint: join.StructLiteral(join.MustTypeName("net/http.Server"), "Handler"),
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
		JoinPoint: join.FunctionBody(join.Function(
			join.Signature(
				[]join.TypeName{join.MustTypeName("net/http.ResponseWriter"), join.MustTypeName("*net/http.Request")},
				nil,
			),
		)),
		Advice: []advice.Advice{
			advice.PrependStmts(code.MustTemplate(
				"{{$arg := .Function.Argument 1}}{{$name := .Function.Name}}instrument.Report({{$arg}}.Context(), instrument.EventStart{{with $name}}, \"name\", {{printf \"%q\" .}}{{end}}, \"verb\", {{$arg}}.Method{{range .DirectiveArgs \"dd:span\"}}, {{printf \"%q\" .Key}}, {{printf \"%q\" .Value}}{{end}})\ndefer instrument.Report({{$arg}}.Context(), instrument.EventEnd{{with $name}}, \"name\", {{printf \"%q\" .}}{{end}}, \"verb\", {{$arg}}.Method{{range .DirectiveArgs \"dd:span\"}}, {{printf \"%q\" .Key}}, {{printf \"%q\" .Value}}{{end}})",
				map[string]string{
					"instrument": "github.com/datadog/orchestrion/instrument",
				},
			)),
		},
	},
}
