# Orchestrion

Automatic instrumentation of Go code

![Orchestrion](https://upload.wikimedia.org/wikipedia/commons/5/55/Welteorchestrion1862.jpg)

## Getting started

1. Install Orchestrion

```sh
go install github.com/datadog/orchestrion
```

2. Let Orchestrion scan the codebase and rewrite it

```sh
orchestrion -w ./
```

3. Check-in the modified code! You might need to run `go get github.com/datadog/orchestrion` and `go mod tidy` if it's the first time you add `orchestrion` to your Go project.

## What it does

Orchestrion processes Go source code and automatically inserts instrumentation.

## How it works

The source code package tree is scanned. For each source code file, use `dave/dst` to build an AST of the source code in the file.

The AST is checked for package level functions or methods that have a `//dd:span` comment attached to them. A function or method annotated with //dd:span must meet an additional condition in order for a span to be automatically inserted into the code. Passing trace information through a Go program requires a context to be present. In order to pass the context through the code, either the first parameter of the function or method must be of type `context.Context` or there must be a parameter of type `*http.Request` (the context can be passed via a field in `*http.Request`). If both conditions are met, the `//dd:span` comment is scanned for tags and code is inserted as the first lines of the function.

Orchestrion also supports automatic tracing of the following libraries:
- [x] `net/http`
- [x] `database/sql`
- [x] `google.golang.org/grpc`

## Next steps

- [ ] Support compile-time auto-instrumentation via `-toolexec`
- [ ] Support auto-instrumenting more third-party libraries
