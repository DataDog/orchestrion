---
title: Aspects
type: docs
weight: 3
---

According to [Wikipedia][wiki-aop]:
> In computing, aspect-oriented programming (AoP) is a programming paradigm that
> aims to increase modularity by allowing the separation of cross-cutting
> concerns. It does so by adding behavior to existing code (an advice) without
> modifying the code, instead separately specifying which code is modified via a
> "pointcut" specification, such as "log all function calls when the function's
> name begins with 'set'". This allows behaviors that are not central to the
> business logic (such as logging) to be added to a program without cluttering
> the code of core functions.

One can easily understand how *Observability* and *Security Monitoring* of an
application are *cross-cutting* concerns... As such, the automatic
instrumentation performed by `orchestrion` is modeled using *aspects*. That
being said, the AoP language is used somewhat loosely by Orchestrion.

## Aspect

An *aspect* is the combination of a *join point* (a *pointcut*, as mentioned in
the Wikipedia definition above, is a set of *join points*) and *advice*.

### Join Points

Orchestrion performs code injection by traversing (and modifying) each and every
Go source file's <abbr title="Abstract Syntax Tree">AST</abbr>. In this
perspective, one can think of a *Join Point* as a function that selects or
rejects an AST node based on its attributes.

Orchestrion evaluates *Join Points* in context, where each node is presented to
these functions with:
- its complete ancestry in the AST, meaning *Join Points* are able to walk up
  the AST to decide whether to select a node or not;
- information about the surrounding package, such as its fully qualified import
  path;
- a configuration object (effectively `string` key-value pairs).

*Join Points* are composable, forming a simple yet versatile language for
addressing AST nodes.

### Advice

*Advice* are functions that transform an AST node. This can mean replacing the
node with something else, modifying certain attributes of the node, or adding
new nodes before or after it.

Orchestrion executes *Advice* on all AST nodes matched by the associated *Join
Point* in post-order; meaning a node is being modified only after all its
children have been considered already. This implies nodes modified by an
*Aspect* are not evaluated by further *Join Points*, and eliminates the risk of
endless recursive instrumentation.

## Next

{{<cards>}}
  {{<card
    link="./join-points"
    title="Join Points"
    icon="book-open"
    subtitle="Join Point reference documentation"
  >}}

  {{<card
    link="./advice"
    title="Advice"
    icon="book-open"
    subtitle="Advice reference documentation"
  >}}

  {{<card
    link="./code-templates"
    title="Code Templates"
    icon="book-open"
    subtitle="Code templates reference"
  >}}

  {{<card
    link="./guidelines"
    title="Aspect Guidelines"
    icon="check-circle"
    subtitle="Guidelines for writing good aspects"
  >}}
{{</cards>}}


[wiki-aop]: https://en.wikipedia.org/wiki/Aspect-oriented_programming
[jp]: ./join-points
[ad]: ./advice
