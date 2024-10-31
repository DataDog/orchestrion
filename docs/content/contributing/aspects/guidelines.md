---
title: Guidelines
type: docs
weight: 10
---

This document privdes guidelines for contributors writing new *Aspects* into
`orchestrion`. Aspect authors should try to adhere to those as much as possible,
as code reviewers may push back on aspect contributions that diverge without a
clear reason.

## Only use exported APIs

While Orchestrion naturally has interest in instrumenting code *everywhere*,
including in private and `/internal/` code, it should avoid doing so by
leveraging private or `/internal/` APIs from the instrumentation libraries as
much as possible.

Using private or `/internal/` APIs from instrumentation libraries creates
excessive coupling to implementation details and increases the risk of automatic
instrumentation resulting in compilation errors when customers upgrade their
transitive dependencies.

An exception to this guideline is when specifically using private or un-exported
details of the module being instrumented; granted this usage is deemed necessary
and there is no alternate solution using only public APIs.

## Minimize changes

Strive to minimize the amount of changes made in the customer's application as
much as possible; as this typically results in reduced risk and reduced
instrumentation overhead.

For example, many libraries are instrumented by modifying some configuration
object, which is then passed to a constructor. In such cases, it is usually best
to instrument around the creation of the configuration value rather than calls
to the constructors.

## Principle of Least Surprise

Don't break the principle of least surprise. This can mean a lot of things, but
some examples follow:

- Don't replace configuration values explicitly set by the customer;
- Don't `panic` unless you **absolutely** must, and even then, probably still
  don't `panic`;
- Don't alter error values unless you are certain customers are not able to
  check them using {{<godoc import-path="errors" name="Is">}} (but it's okay to return other
  errors when an instrumentation feature demands it);

## Reduce, Reuse, Recycle

Avoid copying code across from the instrumentation libraries as much as
possible. Ideally, contribute refactors to the library in order to make it
easier to re-use code in the library's *integrations* in Orchestrion.
