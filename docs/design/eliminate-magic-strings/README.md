# Eliminate Magic Strings: Unified core.Ref System

## Goal

Replace all magic strings in the dnd5e rulebook with typed `core.Ref` values and canonical type constants. This enables:

1. **No magic strings** - every `Source string` that represents a `module:type:id` becomes a `core.Ref`
2. **Typed constants** - use `features.Type`, `conditions.Type` instead of `"features"`, `"conditions"`
3. **Extensibility** - other modules (e.g., `pathfinder2e`) can plug into the rulebook via refs

## Problem Examples

```go
// BAD: Magic strings scattered everywhere
Source: "dnd5e:conditions:raging"           // string that should be Ref
ConditionRef: "dnd5e:conditions:raging"     // same problem
Type: "conditions"                          // should use conditions.Type constant

// GOOD: Typed refs
Source: dnd5e.ConditionRef(conditions.RagingID)  // returns core.Ref
Type: conditions.Type                             // const = "conditions"
```

## Blocking Issue: Import Cycle

Before we can use canonical type constants in `refs.go`, we must break this cycle:

```
combat_test → monster → dnd5e → conditions → combat
```

### Solution: Move Chain Types to events/

Chain types (`DamageChain`, `AttackChain`, stages, etc.) are **event infrastructure**, not combat logic. Moving them to `events/chains.go` breaks the cycle:

```
Before: conditions → combat (cycle when dnd5e → conditions)
After:  conditions → events (no cycle, events is lower-level)
```

## Implementation Phases

### Phase 1: Break the Import Cycle
Move chain types from `combat/` to `events/chains.go`:
- Stage constants (StageBase, StageFeatures, etc.)
- ModifierStages slice
- DamageSourceType and constants
- RerollEvent, DamageComponent structs
- AttackChainEvent, DamageChainEvent structs
- AttackChain, DamageChain typed topics

### Phase 2: Audit Magic Strings
Find and catalog all `Source string` fields that hold ref-like values:
- `RagingCondition.Source`
- `ConditionRemovedEvent.ConditionRef`
- Any other `string` fields containing `module:type:id` patterns

### Phase 3: Convert to core.Ref
Change `Source string` → `Source core.Ref` where appropriate and update all usages.

### Phase 4: Verify Type Constants
Ensure all packages use canonical `Type` constants:
- `features.Type` not `"features"`
- `conditions.Type` not `"conditions"`
- etc.

## Files Reference

See [007-chain-types-refactoring.md](../effect-query-system/007-chain-types-refactoring.md) for detailed chain refactoring steps.

## Success Criteria

1. `go build ./...` succeeds with no import cycles
2. `go test ./...` passes
3. No string literals like `"features"`, `"conditions"` for type references
4. All `Source` fields that represent refs use `core.Ref` type
5. `refs.go` successfully imports and uses domain package `Type` constants
