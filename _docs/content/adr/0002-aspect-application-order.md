# 2. Aspect Application Order

Date: 2025-09-17

## Status

Accepted

## Context

Multiple aspects applied to the same function execute in non-deterministic order, preventing proper instrumentation layering and cross-aspect coordination. This creates issues when integrations need to build upon or coordinate with each other.

### Problem Example: Error Tracking + Span Creation

When both error tracking and span creation aspects target the same function, their execution order significantly impacts effectiveness:

**Current Problematic Order:**

```go
func getError(ctx context.Context) (__result__0 error) {
  var span tracer.Span
  span, ctx = tracer.StartSpanFromContext(ctx, "getError")
  // Error capture happens FIRST because of the defer order
  defer func() {
    span.Finish(tracer.WithError(__result__0))  // Captures unprocessed error
  }()
    // Error processing happens LATER (wrong order), we only have "prepend statements" so we need "defer"
  defer func() {
    __result__0 = errortrace.Wrap(__result__0)  // Processes error after capture
  }()
  // Original function body...
}
```

**Desired Order:**

```go
func getError(ctx context.Context) (__result__0 error) {

  var span tracer.Span
  span, ctx = tracer.StartSpanFromContext(ctx, "getError")
  // Error capture happens LATER because of the defer order
  defer func() {
    span.Finish(tracer.WithError(__result__0))  // Captures processed error
  }()
  // Error processing happens FIRST
  defer func() {
    __result__0 = errortrace.Wrap(__result__0)  // Processes error
  }()
  // Original function body...
}
```

## Decision

We will implement **Integer Ordering with Namespaces** at the **advice level** rather than the aspect level.

### Key Design Decisions

1. **Order at Advice Level**: Apply ordering to individual advice within aspects, providing more granular control than aspect-level ordering.

2. **Namespace Support**: Introduce namespaces to logically group related advice (e.g., "error-handling", "tracing", "metrics") for better coordination.

3. **Backward Compatibility**: Default values (order=0, namespace="default") ensure existing configurations continue to work.

4. **Stable Sort**: Within the same order/namespace, maintain original definition order for predictability.

### Implementation

**Advice Structure:**

- Add `order` field (integer) to advice definitions
- Add `namespace` field (string) for logical grouping
- Sort advice across all matching aspects before application

**Configuration Example:**

```yaml
aspects:
  - id: Error Processing
    advice:
      - prepend-statements:
          namespace: "error-handling"
          order: 10  # Execute first within namespace
          template: |
            defer func() {
              __result__0 = errortrace.Wrap(__result__0)
            }()

  - id: Span Creation
    advice:
      - prepend-statements:
          namespace: "tracing"
          order: 20  # Execute after error handling
          template: |
            span := tracer.StartSpan()
            defer func() {
              span.Finish(tracer.WithError(__result__0))
            }()
```

**Sorting Algorithm:**

1. Sort by namespace alphabetically
2. Within namespace, sort by order (ascending)
3. Within same namespace+order, maintain definition order

### Why Not Aspect-Level Ordering

1. **Granularity**: Aspects can contain multiple advice types - advice-level ordering provides finer control
2. **Flexibility**: Different advice within the same aspect may need different ordering priorities
3. **Composition**: Better supports complex integration scenarios where advice from different aspects need interleaving

## Consequences

### What Becomes Easier

- **Predictable Instrumentation**: Deterministic execution order for cross-aspect coordination
- **Integration Development**: Clear ordering semantics for building complementary features
- **Debugging**: Consistent behavior across different compilation runs
- **Documentation**: Clear examples of proper aspect interaction patterns

### What Becomes More Difficult

- **Configuration Complexity**: Additional fields to consider when writing aspect configurations
- **Coordination**: Teams must agree on namespace conventions and ordering ranges
- **Migration**: Existing configurations may need updates for optimal ordering

### Risks and Mitigations

- **Risk**: Number conflicts between different integration teams
  - **Mitigation**: Namespace separation and documented ordering conventions
- **Risk**: Breaking changes to existing configurations
  - **Mitigation**: Default values maintain backward compatibility
- **Risk**: Increased configuration complexity
  - **Mitigation**: Clear documentation and examples of best practices
