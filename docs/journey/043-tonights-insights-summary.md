# Journey 028: Tonight's Architectural Insights Summary

## Date: 2025-08-16

## Purpose
Capturing all insights from tonight's design session before they're lost. These are the architectural breakthroughs we discovered through discussion and iteration.

## Major Insights

### 1. Everything is Actions and Effects
- **Actions**: Things that activate (rage, cast spell, attack)
- **Effects**: Things that modify game state via event bus
- Pattern: `Action → Execute → Create Effects → Apply to Bus`
- This unifies ALL game mechanics under one pattern

### 2. Actions are Internal Implementation
- **Wrong**: Exposing actions to game server
- **Right**: Actions are hidden inside rulebook
- Public API: `feature.Activate()`, `spell.Cast()`
- Internal: `action.Execute()` with generics for type safety

### 3. Typed Constants Everywhere
```go
// Toolkit provides types
type EventType string
type ResourceKey string
type ModifierSource string

// Rulebook defines constants
const (
    RageUses ResourceKey = "rage_uses"
    AttackRoll EventType = "combat.attack.roll"
)
```
- No magic strings ANYWHERE
- Even toolkit types hint at best practices
- Future toolkit features work with existing constants

### 4. The Loader Pattern Discovery
**Repository → Loader → Orchestrator**

- **Repository**: Just data, no logic
- **Loader**: Data → Domain objects (REUSABLE)
- **Orchestrator**: Complete workflows (NOT reusable pieces)

Key insight: "Orchestrators should not be reusable" - they orchestrate complete workflows using reusable loaders.

### 5. Game Server Reality
- Game server KNOWS it's implementing D&D 5e
- Uses rulebook's typed constants directly
- No dynamic module loading at runtime
- CharacterData is a struct, not JSON

### 6. Feature Loading Pattern
```go
// Features still load from JSON (variable content)
// But character structure is fixed
type CharacterData struct {
    ID       string
    Features []FeatureID      // Not JSON
    Resources map[ResourceKey]int  // Typed!
}
```

### 7. Concentration as Core Pattern
- Concentration is a PATTERN (toolkit level)
- Rules are implementation (rulebook level)
- Same with: duration, targeting, areas, reactions

### 8. The Real Architecture Stack
```
Game Server (knows rulebook)
    ↓
Rulebook Public API (clean interfaces)
    ↓
Rulebook Internal (Action pattern)
    ↓
Toolkit (infrastructure)
```

## Code Patterns We Settled On

### Early Return Errors
```go
// ✅ Good
feature := char.GetFeature(name)
if feature == nil {
    return nil, fmt.Errorf("feature not found: %s", name)
}
// Continue...

// ❌ Bad
if feature := char.GetFeature(name); feature != nil {
    // nested logic
}
```

### Input/Output Everywhere
```go
// ✅ Always
func LoadCharacter(ctx context.Context, input *LoadInput) (*LoadOutput, error)

// ❌ Never
func LoadCharacter(ctx context.Context, id string) (*Character, error)
```

### Generic Actions for Type Safety
```go
// Internal implementation
type RageAction struct{}
// Implements Action[EmptyInput]

type BlessAction struct{}
// Implements Action[CastInput]

// No type assertions needed!
```

## What Makes This Architecture Work

1. **Clean boundaries** - Each layer has clear responsibilities
2. **Type safety** - Constants and generics prevent errors
3. **Hidden complexity** - Public API is simple, complexity is internal
4. **Extensibility** - New features/spells plug in easily
5. **Testability** - Mock at interface boundaries

## Implementation Roadmap

### Phase 1: Foundation
1. Add base types to toolkit (ResourceKey, ModifierSource, etc.)
2. Implement generic Action[T] interface
3. Create standard input types (EmptyInput, TargetedInput, etc.)

### Phase 2: Rulebook Structure
1. Define typed constants in rulebook
2. Create public interfaces (Character, Feature, Spell)
3. Implement rage with internal action pattern
4. Test the pattern with bless (concentration + targeting)

### Phase 3: Loader Pattern
1. Create internal/loaders directory structure
2. Implement CharacterLoader
3. Refactor orchestrators to use loaders
4. Simplify repositories to just data

### Phase 4: Integration
1. Wire up to rpg-api
2. Test complete flow from API to effects
3. Validate the patterns scale

## Open Questions Remaining

1. **Action Registry** - How to store different Action[T] types?
2. **Effect Lifecycle** - Duration, cleanup, persistence?
3. **State Serialization** - How to save/load active effects?

## Key Decisions Made

1. ✅ Actions are internal only
2. ✅ Everything uses typed constants
3. ✅ Loaders are separate from repositories
4. ✅ Orchestrators do complete jobs
5. ✅ Game server knows its rulebook
6. ✅ Character structure is fixed (features are variable)

## Why This Matters

We've designed an architecture where:
- The toolkit knows nothing about games
- Games implement rules using toolkit patterns
- Everything is data-driven and type-safe
- New content doesn't require code changes
- The public API is clean and intuitive

## Tomorrow's First Task

Create the Action[T] interface in toolkit with proper generics. This is the foundation everything else builds on.

---

*This document captures our 8+ hour design session. The architecture evolved significantly but we've arrived at something clean, extensible, and type-safe. The Actions/Effects pattern combined with the Loader pattern gives us the right abstractions at the right layers.*