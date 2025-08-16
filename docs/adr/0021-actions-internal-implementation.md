# ADR-0021: Actions as Internal Implementation Pattern

## Status
Proposed

## Context

We discovered that features, spells, and abilities all follow a similar execution pattern:
- Check resources/prerequisites
- Create effects
- Apply effects to event bus
- Handle results

Initially, we considered exposing Action interfaces to the game server, but realized this was leaking implementation details.

## Decision

**Actions are an internal implementation pattern within rulebooks, not part of the public API.**

### Public API (What Game Server Sees)
```go
type Feature interface {
    Activate(ctx context.Context) error
}

type Spell interface {
    Cast(ctx context.Context, input CastInput) error
}
```

### Internal Implementation (Hidden)
```go
type RageFeature struct {
    action Action[EmptyInput]  // Internal, not exposed
}

func (f *RageFeature) Activate(ctx context.Context) error {
    return f.action.Execute(ctx, f.owner, EmptyInput{}, f.bus)
}
```

## Consequences

### Positive
- Clean, intuitive public API
- Type safety through generics (internally)
- Consistent implementation pattern
- Can change internal implementation without breaking API
- Game server code reads naturally

### Negative
- Two layers of abstraction (public interface + internal action)
- Need to maintain both public and internal interfaces

### Neutral
- Actions become our internal consistency tool
- Each feature/spell decides its own Action type

## Implementation Notes

1. Toolkit provides generic Action[T] interface
2. Rulebooks implement actions internally
3. Public interfaces hide action complexity
4. Game server never imports action types