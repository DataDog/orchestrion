---
title: Code Templates
type: docs
weight: 3
---

Code templates are commonly used in _Advice_. They provide a simple way to
create new AST nodes that will be injected into the instrumented file, allowing
references to the matched node.

## Generalities

### Template Syntax

Orchestrion code templates are rendered using the Go standard library
{{<godoc "text/template">}} module. Refer to the module's documentation to learn
about the general syntax of these template values.

In addition to the template text, Orchestrion code templates allow for two
additional configuration objects: `imports` and `links`.

### Imports

The `imports` map binds Go language identifiers to package import paths. All
entities (functions, types, etc...) that are used in the template text and
which are not local to the instrumented package **must** be accessed through the
identifier from the `imports` map, even if that package is already imported by
the instrumented file.

See the following examples:

```yaml
template: fmt.Println({{ . }})
imports:
  # The template uses `fmt` so we bind it to the identifier `fmt` below.
  fmt: fmt
```

```yaml
template: |-
  tracer.Start()
  defer tracer.Stop()
imports:
  tracer: gopkg.in/DataDog/dd-trace-go.v1/ddtrace/tracer
```

### Links

The `links` list is similar to the `imports` map except it does not bind import
paths to identifiers. Instead, it contains a list of packages that are required
to be presented to the _linker_ in order to satisfy dependencies of the code
rendered by the template.

This is typically only necessary when the template produces `//go:linkname`
directives that are satisfied by foreign compilation units.

## Template context

This section documents what functions are available for use in template text, as
well as what the `.` context value represents, and which methods are
available for use on it. These are essential for authoring Orchestrion code
templates.

### The `Version` function

The simple `Version` function returns the version of Orchestrion being used, as
represented by the version tag name.

* Given:
  ```go-template
  fmt.Println("Orchestrion version is: {{ Version }}")
  ```
* The output would look like:
  ```go
  fmt.Println("Orchestrion version is: v0.7.3")
  ```

### The value of `.`

In orchestrion code templates, `.` is a value that represents information about
the node matched by the *join point* that led to evaluation of the current
*advice*.

Simply using `{{ . }}` results in *moving* the current AST node into the
template's output. The *moved* AST node retains its original source position
information, which is essential to avoid unnecessarily affecting the
application's stack trace information.

{{<callout type="important">}}
A given AST node may be present only **exactly once** in the resulting AST, so
template authors must be careful to avoid producing the same node in multiple
places.

The `.AST` method can be used to *copy* nodes (instead of moving them), making
them safe to produce in multiple locations in the output.
{{</callout>}}

#### The `.AST` method

The `.AST` method returns a view of the node's AST. This view allows refering
to all attributes of the original AST nodes, producing a similar view object for
each child node.

When view objects are rendered by the template, they produce a *copy* of the
view's underlying node. The **copied** AST nodes are detached from the original
node's source location, making them appear as synthetic in the produced output,
meaning they would be reported as belonging to generated code in the
application's stack trace.

The complete type hierarchy for these views corresponds to the
{{<godoc "github.com/dave/dst" "Node">}} implementations, which provide the
underlying AST model. Node properties can be accessed using their usual name
from {{<godoc "github.com/dave/dst">}}.

#### The `.DirectiveArgs` method

A *directive* is a specially formatted single-line comment, without white space
between the `//` and the *directive* name, similar to the `//go:linkname`
directive.

The `.DirectiveArgs` method can be used to look up the AST in order to locate a
given *directive*, and to extract key-value pair arguments from it. It requires
a directive name argument, and returns the list of values that have `.Key` and
`.Value` fields, respectively allowing access to the argument's key and value.

Here is an example of how this can be used, from the instrumentation of
`//dd:span` directives:

```go-template
{{- $ctx := .FindArgument "context.Context" -}}
{{- $name := .Function.Name -}}
{{$ctx}} = instrument.Report({{$ctx}}, event.EventStart{{with $name}}, "function-name", {{printf "%q" .}}{{end}}
{{- range .DirectiveArgs "dd:span" -}}
  , {{printf "%q" .Key}}, {{printf "%q" .Value}}
{{- end -}})
defer instrument.Report({{$ctx}}, event.EventEnd{{with $name}}, "function-name", {{printf "%q" .}}{{end}}
{{- range .DirectiveArgs "dd:span" -}}
  , {{printf "%q" .Key}}, {{printf "%q" .Value}}
{{- end -}})
```

#### The `.FindArgument` method

The `.FindArgument` method requires a single type name argument, and walks up
the AST, looking for an argument of that time on the signature of an enclosing
function. If such an argument is found, it returns the name of the closest found
argument of the required type. Otherwise it returns an empty string.

Here is an example of its usage to locate a `context.Context` argument:
```go-template
{{- $ctx := .FindArgument "context.Context" -}}
```

#### The `.Function` method

The `.Function` method walks up the AST to find the closest enclosing function.
This can be used within a template to access the enclosing function's name,
parameters or return values.

It accepts no argument and returns an object that ptovides access to the
function's signature details:

- `.Function.Name` returns the function's name, or a blank string if the
  function is a function literal expression.
- `.Function.Argument n` returns the name of the `n`th argument (`0`-based) of
  the function; and implicitly assigns it a name if the argument was anonymous
  or named `_`. Returns an error if the surrounding function does not have
  enough arguments.
- `Function.Returns n` returns the name of the `n`th return value (`0`-based) of
  the function; and implicitly assigns it a name if the return value was
  anonymous or named `_`. Returns an error if the surrounding function does not
  have enough return values.
- `Function.Receiver` returns the name of the receiver value for this function.
  Returns an error if the surrounding function is not a method.

## Next

{{<cards>}}
  {{<card
    link="../guidelines"
    title="Aspect Guidelines"
    icon="check-circle"
    subtitle="Guidelines for writing good aspects"
  >}}
{{</cards>}}
