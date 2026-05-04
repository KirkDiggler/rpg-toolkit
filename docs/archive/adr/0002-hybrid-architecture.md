# ADR-0002: Hybrid Event-Driven Architecture

Date: 2025-01-03

## Status

Accepted

## Context

When designing the RPG Toolkit, we evaluated several architectural patterns:

1. **Entity Component System (ECS)**: Popular in game engines, where entities are just IDs, components hold data, and systems process entities with specific components
2. **Event Sourcing**: All state changes are events, current state is derived from replaying events
3. **Traditional OOP**: Classes with inheritance hierarchies for game objects
4. **Service-Oriented**: Standalone services communicating through APIs

### Requirements

- Support multiple RPG systems (D&D, Pathfinder, etc.)
- Enable mod-like extensibility without modifying core code
- Maintain testability and clarity
- Avoid over-engineering for turn-based gameplay
- Allow different storage backends

## Decision

We will use a **hybrid event-driven architecture** that combines:

1. **Traditional module organization** for clarity
2. **Event-driven communication** between modules for decoupling
3. **Interface-based design** for extensibility

### Architecture Overview

```
┌─────────────────┐     ┌─────────────────┐     ┌─────────────────┐
│   Game Module   │     │ Condition Module│     │  Combat Module  │
│                 │     │                 │     │                 │
│ SubscribeFunc() ├────►│ SubscribeFunc() ├────►│ SubscribeFunc() │
└────────┬────────┘     └────────┬────────┘     └────────┬────────┘
         │                       │                         │
         └───────────────────────┴─────────────────────────┘
                                 │
                         ┌───────▼────────┐
                         │   Event Bus    │
                         │                │
                         │ Publish(event) │
                         └────────────────┘
```

### Key Principles

1. **Modules communicate through events**, not direct calls
2. **Features compose through event handlers**, not inheritance
3. **Game rules are data + handlers**, not hard-coded logic
4. **Storage is behind interfaces**, not baked into modules

## Consequences

### Positive

- **Extensibility**: New features can be added without modifying existing code
- **Testability**: Modules can be tested in isolation with mock events
- **Clarity**: Traditional module structure is familiar to developers
- **Flexibility**: Different game systems can subscribe to different events
- **Performance**: Turn-based games don't need ECS optimization

### Negative

- **Event debugging**: Following event flow can be harder than direct calls
- **Ordering complexity**: Handler priority needs careful management
- **Learning curve**: Event-driven patterns may be unfamiliar to some
- **Potential overhead**: Events have more overhead than direct calls

### Neutral

- Not suitable for real-time games requiring ECS performance
- Requires discipline to avoid coupling through event data
- Event schema evolution needs consideration

## Example

```go
// A rage feature is just an event subscriber
type RageFeature struct {
    eventBus events.Bus
}

func (r *RageFeature) Initialize() {
    // Subscribe to damage calculations
    r.eventBus.SubscribeFunc(events.EventCalculateDamage, 100, r.addRageDamage)
}

func (r *RageFeature) addRageDamage(ctx context.Context, e events.Event) error {
    if !r.isRaging(e.Source()) {
        return nil
    }
    
    // Add rage bonus through the event
    e.Context().AddModifier(events.NewModifier(
        "rage",
        events.ModifierDamageBonus,
        events.NewRawValue(2, "rage"),
        100,
    ))
    
    return nil
}
```

## Alternatives Considered

### Pure ECS
- ✅ Maximum flexibility and performance
- ❌ Overkill for turn-based games
- ❌ Steep learning curve
- ❌ Everything becomes arrays and indices

### Pure Event Sourcing
- ✅ Perfect audit trail
- ✅ Time travel debugging
- ❌ Complex state reconstruction
- ❌ Storage requirements grow unbounded

### Traditional OOP
- ✅ Familiar to most developers
- ❌ Inheritance hierarchies become brittle
- ❌ Hard to add cross-cutting features
- ❌ Tight coupling between components

## References

- [Game Programming Patterns - Component](https://gameprogrammingpatterns.com/component.html)
- [Martin Fowler - Event Sourcing](https://martinfowler.com/eaaDev/EventSourcing.html)
- [Go Proverbs - Interface composition](https://go-proverbs.github.io/)