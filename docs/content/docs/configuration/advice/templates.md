---
title: Code Templates
weight: 1
icon: code
---

## Introduction

The majority of advices result in the addition of new code _before_, _after_, or
_around_ existing code. Join points allow a given _aspect_ to select a
particular node in the source's <abbr title="Abstract Syntax Tree">AST</abbr> as
the place where new code needs to be inserted.

In order to specify the new code as naturally as possible, Orchestrion uses text
templates that are expanded using the Go standard library's
[`text/template` package][go-text-template]. This documentation contains several
examples of such templates, but we recommend you refer to the Go standard
library documentation for more information about the capabilities and syntax of
the Go template language.

Orchestrion _code templates_ must expand to _valid_ Go code, composed of one or
more statements (certain advices, such as `wrap-expression` have more strict
requirements), and is parsed in the context of an arbitrary function (meaning it
may not contain top-level-only declarations, such as `import` declarations).

## Schema

```yaml
imports: # map[string]string
  symbol: import/path
  other: other/import/path/v3
template: # string
  go text template
```

## Import Mapping

All Orchestrion code templates are composed of a Go text template paired with an
imports map. The imports map binds symbol names with an import path, such that
all occurrences of a symbol in the map will be qualified with the relevant
import path.

The symbol names in the import map will be mangled if necessary to avoid causing
naming collisions with other identifiers in the same scopes.

## Evaluation Context

### The value of `.`

When templates are evaluated, `.` is set to a value allowing access to the AST
node that was selected by the configured _join point_. This exposes a number of
methods that help compose useful templates.

#### `.Assignment`

The `.Assignment` method returns a value representing the closest assignment
statement in the ancestry of `.`. It returns `nil` in case no assignment exists
up the node tree.

The returned value allows access to the assignment's left-hand side expression
corresponding to the index at which `.` is found. In cases where the asisgnment
has a single left-hand side expression, this is very straight forward; but in
cases when multiple values are assigned, it becomes more interesting:

```go
//                 ╭── .
//             ╭───┴──╮
   foo, bar := "string", 1337.42
// ╰┬╯
//  ╰── .Assignment

//                          ╭── .
//                       ╭──┴──╮
   foo, bar := "string", 1337.42
//      ╰┬╯
//       ╰── .Assignment
```

Assignments of functions which return multiple values are not supported at this
point (this may be supported in the future):

```go
//                         ╭── .
//              ╭──────────┴──────────╮
   file, err := os.Open("/dev/urandom")
//
// 💥 .Assignment -- error
```

##### Example

Assuming the given template context:
```go
   foo, bar := "string", /* HERE --> */ 1337.42
```

{{<tabs items="Template,Expanded">}}
  {{<tab>}}
    ```go-text-template
    {{ .Assignment.LHS }} = {{ . }} * 2
    ```
  {{</tab>}}
  {{<tab>}}
    ```go
    foo = 1337.42 * 2
    ```
  {{</tab>}}
{{</tabs>}}


#### `.AST`

The `.AST` method allows navigating the AST nodes from [`dave/dst`][dave-dst] so
you can extract information from the node and its children as appropriate. Refer
to the [`dave/dst`][dave-dst] documentation for information about the AST nodes
structure.

Nodes accessed though the `.AST` method will be _copied_ (instead of _moved_) to
the updated AST, meaning they will be treated as _synthetic_ nodes, resulting in
their source file and line information being lost.

##### Example

In the context of:
```go
func Example() {
  db, err := /* HERE -> */ sql.Open("driver-name", "dsn")
  if err != nil {
    panic(err)
  }
  defer db.Close()
  // ...
}
```

{{<tabs items="Template,Expanded">}}
  {{<tab>}}
    ```go-text-template
    sqltrace.Open(
      {{ range .AST.Args }}{{ . }},
    {{ end }})
    ```
  {{</tab>}}
  {{<tab>}}
    ```go
    sqltrace.Open(
      "driver-name",
      "dsn",
    )
    ```
  {{</tab>}}
{{</tabs>}}

#### `.DirectiveArgs`

The `.DirectiveArgs` method allows access to special directive comments, which are
single-line comments led by `//`, immediately followed by the directive name
without any white space in between:

```go
//dd:span tag:value other:tag
func correctUseOfDirective() { /* ... */ }

// dd:ignore 💥 space between // and dd:ignore prevents this from being a directive
func incorrect() { /* ... */ }
```

The `.DirectiveArgs` method returns a list of key-value parameters passed to the
directive comment, which is empty if there was no such directive, or if the
directive had no arguments.

##### Example

In the following context:
```go
//dd:span foo:bar baz:bat
func foo() {
  /* HERE */
}
```

{{<tabs items="Template,Expanded">}}
  {{<tab>}}
    ```go-text-template
    {{- range .DirectiveArgs "dd:span" }}
    fmt.Println("Key = {{ .Key }}, Value = {{ .Value }}")
    {{ end -}}
    ```
  {{</tab>}}
  {{<tab>}}
    ```go
    fmt.Println("Key = foo, Value = bar")
    fmt.Println("Key = baz, Value = bat")
    ```
  {{</tab>}}
{{</tabs>}}

#### `.FindArgument`

The `.FindArgument` method can be used to obtain the name of the closest
function argument of a specified type. This is often used to access a
`context.Context` value. It returns an empty string if no parameter of the
specified type exists.

It expects a single argument representing the qualified type name of the
argument being looked for. While this looks like Go syntax, the type name is
qualified using the import path rather than the package name:

##### Example

In the context of:
```go
func example(ctx context.Context, idx int) {
  closure := func(r *http.ResponseWriter) {
    /* HERE */
  }
  // ...
}
```

{{<tabs items="Template,Expanded">}}
  {{<tab>}}
    ```go-text-template
    fmt.Println("ResponseWriter = {{ .FindArgument "*net/http.ResponseWriter" }}")
    fmt.Println("Context = {{ .FindArgument "context.Context" }}")
    fmt.Println("Integer = {{ .FindArgument "int" }}")
    ```
  {{</tab>}}
  {{<tab>}}
    ```go
    fmt.Println("ResponseWriter = r")
    fmt.Println("Context = ctx")
    fmt.Println("Integer = idx")
    ```
  {{</tab>}}
{{</tabs>}}

#### `.Function`

The `.Function` method returns a helper that allows accessing basic information
about the surrounding function's declaration. It exposes:
- `.Name` &ndash; the name of the function, or blank if it's a function literal
  expression;
- `.Argument(n int)` &ndash; the name of the `n`th argument of the function,
  automatically assigning an identifier to anonymous parameters;
- `.Returns(n int)` &ndash; the name of the `n`th return value of the function,
  automatically assigning an identifier to anonymous return values.

##### Example

In the context of:
```go
func Example(a, b int) error {
  closure := func(c, d string, e bool) (any, error) {
    /* HERE */
  }
  // ...
}
```

{{<tabs items="Template,Expanded">}}
  {{<tab>}}
    ```go-text-template
    fmt.Println("Function name: {{ .Function.Name }}")
    fmt.Println("Arg 2: {{ .Function.Argument 2 }}")
    fmt.Println("Return 0: {{ .Function.NamReturns 0 }}")
    ```
  {{</tab>}}
  {{<tab>}}
    ```go
    fmt.Println("Function name: ")
    fmt.Println("Arg 2: e")
    fmt.Println("Return 0: __returns__0")
    ```
  {{</tab>}}
{{</tabs>}}


### The `Version` function

In cases where Orchestrion's version number is necessary, a `Version` function
is available in the templates' evaluation context. It returns Orcherstrion's
version number (e.g, `v0.7.0-dev`).

#### Example

{{<tabs items="Template,Expanded">}}
  {{<tab>}}
    ```go-text-template
    fmt.Println("Orchestion version is {{ Version }}")
    ```
  {{</tab>}}
  {{<tab>}}
    ```go
    fmt.Println("Orchestion version is v0.7.0-dev")
    ```
  {{</tab>}}
{{</tabs>}}

<!-- References -->
[go-text-template]: https://pkg.go.dev/text/template
[dave-dst]: https://pkg.go.dev/github.com/dave/dst#Node
