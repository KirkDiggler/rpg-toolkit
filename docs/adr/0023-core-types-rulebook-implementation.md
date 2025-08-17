# ADR-0023: Core Provides Types, Rulebooks Provide Implementation

## Status
Accepted

## Context

While implementing the D&D 5e rage feature, we discovered fundamental architectural issues:

1. **String proliferation**: Magic strings everywhere (`"advantage"`, `"strength_check"`, `"damage.bludgeoning"`)
2. **Reinventing infrastructure**: Not using `effects.Core` that already exists
3. **Mixed responsibilities**: Features trying to be conditions, conditions trying to be entities
4. **Missing type safety**: Everything is strings instead of typed constants

The attempt revealed that we're fighting against missing infrastructure rather than implementing game logic.

## Decision

### Core Layer Provides Types Only

The `rpg-toolkit/core` package should define:
- Type definitions and interfaces
- Constants for common concepts
- No implementation, just contracts

```go
// core/modifiers.go
type ModifierType string
type ModifierTarget string
type ModifierSource string

const (
    ModifierAdvantage    ModifierType = "advantage"
    ModifierDisadvantage ModifierType = "disadvantage"
    ModifierResistance   ModifierType = "resistance"
    ModifierImmunity     ModifierType = "immunity"
    ModifierBonus        ModifierType = "bonus"
    ModifierPenalty      ModifierType = "penalty"
)

// core/combat/types.go  
type DamageType string
const (
    DamageBludgeoning DamageType = "bludgeoning"
    DamagePiercing    DamageType = "piercing"
    DamageSlashing    DamageType = "slashing"
    DamageFire        DamageType = "fire"
    // ... etc
)
```

### Rulebooks Define Their Specific Needs

Each rulebook (dnd5e, pathfinder, etc.) defines:
- Their specific modifier targets
- Their specific event types
- Their specific conditions/effects

```go
// rulebooks/dnd5e/modifiers.go
const (
    TargetStrengthCheck = core.ModifierTarget("ability.strength.check")
    TargetStrengthSave  = core.ModifierTarget("ability.strength.save")
    TargetACBonus       = core.ModifierTarget("ac.bonus")
)

// rulebooks/dnd5e/combat/events.go
var (
    TurnStartRef = core.MustParseRef("dnd5e:combat:turn-start")
    TurnEndRef   = core.MustParseRef("dnd5e:combat:turn-end")
)
```

### Effects Infrastructure Is The Foundation

Conditions are just effects with lifecycle:
```go
// rulebooks/dnd5e/conditions/rage.go
type RageEffect struct {
    *effects.Core  // Provides base functionality
    level int
    // rage-specific tracking
}

// The feature just manages activation
type RageFeature struct {
    uses int
    effectManager *effects.Manager
}

func (f *RageFeature) Activate() error {
    effect := NewRageEffect(f.level)
    return f.effectManager.Apply(effect)
}
```

## Consequences

### Positive
- **Type safety**: No more magic strings
- **Clear boundaries**: Core defines types, rulebooks implement
- **Reusable infrastructure**: effects.Core handles common patterns
- **Game-specific flexibility**: Each rulebook defines what it needs
- **Easier testing**: Can test against types, not strings

### Negative  
- **More files**: Types spread across core and rulebooks
- **Migration effort**: Existing code needs updating
- **Learning curve**: Developers need to understand the boundary

### Neutral
- **Explicit over implicit**: Must declare types before use
- **No central registry**: Each rulebook manages its own types

## Implementation Order

1. Create core type definitions (Issue #222)
2. Create combat event types (Issue #223)  
3. Refactor features to use effects.Core (Issue #224)
4. Create conditions manager tool (Issue #225)
5. Migrate existing features (Issue #226)

## Success Metrics

Success is when:
- No magic strings in feature implementations
- Features are <100 lines of actual game logic
- Adding a new condition requires only defining its unique behavior
- Type mismatches are caught at compile time, not runtime

## Example: Rage Implementation After This Change

```go
// ~50 lines of actual game logic
type RageFeature struct {
    *features.Simple  // Base implementation
    uses int
}

func (f *RageFeature) Activate(owner core.Entity) error {
    // Just activate the effect
    effect := NewRageEffect(owner, f.level)
    return f.bus.Publish(&EffectAppliedEvent{
        Effect: effect,
        Target: owner,
    })
}

type RageEffect struct {
    *effects.Core
    damageBonus int
}

func (e *RageEffect) Modifiers() []core.Modifier {
    return []core.Modifier{
        {Type: core.ModifierAdvantage, Target: dnd5e.TargetStrengthCheck},
        {Type: core.ModifierAdvantage, Target: dnd5e.TargetStrengthSave},
        {Type: core.ModifierResistance, Target: dnd5e.TargetDamageBludgeoning},
        {Type: core.ModifierBonus, Target: dnd5e.TargetDamageMelee, Value: e.damageBonus},
    }
}
```

## Note on Combat

Combat is particularly tricky because:
- Turn order matters
- Actions have timing (reactions, bonus actions)
- Effects interact (advantage/disadvantage cancel)
- State changes rapidly

We must be deliberate and thoughtful. Better to have no combat system than a broken one.