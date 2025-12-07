# Effect Query System Design

This folder contains the design exploration for the Effect Query System - a unified approach to handling game effects, queries, and context.

## Documents

| Doc | Status | Description |
|-----|--------|-------------|
| [001-problem-statement.md](001-problem-statement.md) | Complete | The problems we're solving |
| [002-query-topics.md](002-query-topics.md) | Draft | Query chains for attribute breakdowns |
| [003-context-pattern.md](003-context-pattern.md) | Draft | Enriched context for situational data |
| [004-hybrid-design.md](004-hybrid-design.md) | Draft | **Recommended**: Combining both approaches |
| [005-core-effect-interface.md](005-core-effect-interface.md) | Draft | Simplified core.Effect design |
| [006-encounter-package.md](006-encounter-package.md) | Early | Future: tools/encounter for turn lifecycle |

## Key Decisions

### 1. Hybrid Approach (Context + Query Topics)
- **Context**: For situational data (positions, allies, encounter state)
- **Query Topics**: For computed values with breakdowns (AC, saves)

### 2. Simple Core Effect
```go
type Effect interface {
    Apply(ctx context.Context) error
    Remove(ctx context.Context) error
    IsActive() bool
}
```
No generic. Target is implementation detail.

### 3. Persistence Separate from Effect
```go
type Persistable interface {
    ToJSON() (json.RawMessage, error)
}
```
Effects optionally implement this.

### 4. BusEffect for Event-Based Effects
```go
type BusEffect interface {
    Effect
    SetBus(bus events.EventBus)
}
```
Most conditions are BusEffects.

## The Patterns

### Query Pattern (for "what's my AC?")
```
Character.GetAC()
  → publishes ACQueryEvent to ACQueryTopic
  → all conditions subscribed to ACQueryTopic add contributions
  → chain executes, returns ACResult with breakdown
```

### Context Pattern (for "is ally adjacent?")
```
CombatResolver sets up GameContext with EncounterContext
  → attaches to context.Context
  → publishes attack events
  → SneakAttack checks gameCtx.Encounter.HasAllyAdjacentTo()
```

### Effect Lifecycle Pattern
```
manager.ApplyCondition(ctx, ragingCondition)
  → condition.SetBus(bus)
  → condition.Apply(ctx)
    → subscribes to DamageChain, TurnEnd, etc.
  → manager tracks active condition

// Later...
manager.RemoveCondition(ctx, "raging")
  → condition.Remove(ctx)
    → unsubscribes from all topics
```

## Next Steps

1. [ ] Review and refine documents based on discussion
2. [ ] Decide on open questions in each document
3. [ ] Implement `core/effect` package updates
4. [ ] Implement `rulebooks/dnd5e/queries/` package
5. [ ] Implement `rulebooks/dnd5e/context/` package
6. [ ] Update one condition as proof of concept
7. [ ] Test with AC query end-to-end

## Open Questions

1. Should `BaseBusEffect` live in core or rulebook?
2. Do we need typed query stages or reuse combat stages?
3. How to handle query caching/invalidation?
4. Should effects declare what they modify (static) or just subscribe (dynamic)?
