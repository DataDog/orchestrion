module github.com/datadog/orchestrion/internal/injector/builtin/generator

go 1.19

require (
	github.com/datadog/orchestrion v0.0.0-00010101000000-000000000000
	github.com/dave/jennifer v1.7.0
	golang.org/x/tools v0.17.0
	gopkg.in/yaml.v3 v3.0.1
)

require (
	github.com/dave/dst v0.27.2 // indirect
	github.com/kr/text v0.2.0 // indirect
	golang.org/x/mod v0.14.0 // indirect
	gopkg.in/check.v1 v1.0.0-20201130134442-10cb98267c6c // indirect
)

replace github.com/datadog/orchestrion => ../../../..
