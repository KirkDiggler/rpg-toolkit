# ADR-0005: Effect Composition Pattern

**Status**: Proposed  
**Date**: 2024-01-04  
**Author**: Kirk Diggler

## Context

As we implement various game mechanics (conditions, proficiencies, resources, features, equipment effects), we've identified significant code duplication and similar patterns across these systems. All of these mechanics:

1. Modify game behavior through the event system
2. Have apply/remove lifecycles
3. Track event subscriptions
4. Need activation/deactivation logic
5. Associate with entities (as owner, target, etc.)

Initially, we considered extracting a shared base `Effect` class to eliminate duplication. However, this approach would:
- Reduce domain clarity (everything becomes an "Effect")
- Force premature generalization
- Create rigid inheritance hierarchies
- Provide minimal benefit for the complexity added

## Decision

We will use a **composition-based approach** where game mechanics remain distinct domain types but share common behavioral components.

### Core Components

1. **EffectCore**: Basic subscription management and lifecycle
   - ID and type management
   - Active/inactive state
   - Event subscription tracking
   - Apply/Remove lifecycle

2. **Behavioral Interfaces**: Composable behaviors that effects can implement
   - `ConditionalEffect`: Only applies under certain conditions
   - `ResourceConsumer`: Consumes limited resources  
   - `TemporaryEffect`: Has duration/expiration
   - `StackableEffect`: Defines stacking rules
   - `DiceModifier`: Adds dice expressions to rolls (e.g., "+1d4")
   - Additional behaviors as needed

3. **Domain Types**: Specific game mechanics that compose these behaviors
   - `Proficiency`: EffectCore + optional Conditional
   - `Condition`: EffectCore + optional Temporary + Stackable
   - `Feature`: EffectCore + optional Conditional + ResourceConsumer
   - `Resource`: Different pattern focused on numerical tracking

### Implementation Pattern

```go
// Shared core functionality
type EffectCore struct {
    id     string
    typ    string
    source string
    active bool
    tracker SubscriptionTracker
}

// Domain-specific type composes behaviors
type SimpleProficiency struct {
    EffectCore
    owner   core.Entity
    subject string
    
    // Optional composed behaviors
    conditional ConditionalEffect
}

// Behaviors are interfaces with focused responsibilities
type ConditionalEffect interface {
    CheckCondition(ctx Context) bool
}
```

## Consequences

### Positive

1. **Domain Clarity**: Game concepts remain clear (Proficiency vs Condition)
2. **Flexible Composition**: New mechanics are combinations of behaviors
3. **Reusable Components**: Each behavior written once, used everywhere
4. **Independent Testing**: Behaviors can be tested in isolation
5. **Open/Closed**: New behaviors added without modifying existing code
6. **Event Bus Integration**: Effects remain active participants in game flow

### Negative

1. **More Interfaces**: Multiple small interfaces instead of one large one
2. **Composition Complexity**: Developers must understand composition patterns
3. **Potential Over-Engineering**: Simple effects might not need all features

### Neutral

1. **Gradual Migration**: Existing code can be migrated incrementally
2. **Learning Curve**: New pattern to understand, but follows Go idioms
3. **Documentation Needs**: Requires clear examples of composition

## Implementation Notes

1. Start with `EffectCore` and `SubscriptionTracker` in mechanics/common
2. Add behavioral interfaces as needed (YAGNI principle)
3. New systems (resources, features) use this pattern from start
4. Migrate existing systems (conditions, proficiencies) gradually
5. Document composition patterns with examples

## Alternatives Considered

1. **Shared Base Class**: Traditional inheritance approach
   - Rejected: Too rigid, loses domain clarity

2. **Copy-Paste**: Accept duplication
   - Rejected: Maintenance burden will grow

3. **Code Generation**: Generate boilerplate
   - Rejected: Adds complexity, hides behavior

4. **No Abstraction**: Keep everything separate
   - Rejected: Pattern is too common to ignore

## References

- Journey 005: Effect Composition Pattern
- ADR-0002: Hybrid Architecture (event-driven design)
- ADR-0003: Conditions as Entities (entity pattern)
- Go Effective Go: Interface composition