# D&D 5e Rulebook Development Guidelines

This document contains patterns specific to the dnd5e rulebook module.

## Refs Pattern

**Everything has a Ref.** All identifiable content (features, conditions, combat abilities, actions, weapons, etc.) must have a `*core.Ref` with `{Module, Type, ID}`.

- Module: `"dnd5e"` (enables future modules like `"artificer"` without ID conflicts)
- Type: Category (`"features"`, `"conditions"`, `"actions"`, etc.)
- ID: Specific identifier (`"rage"`, `"dodging"`, `"strike"`)

**Game server uses Refs** to identify what to activate/check. Proto enum maps to Ref.

**Loaders reconstitute behavior.** Refs identify the module/package, then:
- `LoadFromData(data)` - When types share a consistent schema (e.g., actions)
- `LoadJSON(data)` - When types have unique state structures (e.g., features, conditions)

Choose based on whether the data structure is homogeneous or heterogeneous.

**Refs namespace pattern** (`refs/` package):
```go
refs.Features.Rage()           // *core.Ref for Rage feature
refs.Conditions.Dodging()      // *core.Ref for Dodging condition
refs.CombatAbilities.Attack()  // *core.Ref for Attack ability
refs.Actions.Strike()          // *core.Ref for Strike action
```

## Conditions vs Effects

Conditions are really "effects" but we're saving that rename for 1.0. Use `character.HasCondition(refs.Conditions.Dodging())` to check for active effects like Dodging or Disengaging.

## Two-Level Action Economy

D&D 5e has two levels of resource consumption:

1. **Action Economy** - What you spend (action, bonus action, reaction)
2. **Capacity** - What you get to do (attacks, movement, off-hand attacks, flurry strikes)

Example: Taking the Attack ability (spends action) grants attacks (capacity). Each Strike action consumes one attack from that capacity.

Key fields in `ActionEconomy`:
- Primary: `ActionsRemaining`, `BonusActionsRemaining`, `ReactionsRemaining`
- Capacity: `AttacksRemaining`, `MovementRemaining`, `OffHandAttacksRemaining`, `FlurryStrikesRemaining`
