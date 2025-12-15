# Chain Types Refactoring: Breaking the Import Cycle

## Goal

Fix the import cycle in the dnd5e rulebook that prevented `refs.go` from using canonical `Type` constants from domain packages.

### The Cycle

```
combat_test → monster → dnd5e → conditions → combat
```

### Root Cause

The `conditions` package imported `combat` package for chain types (`DamageChain`, `DamageChainEvent`, stages, etc.), but `dnd5e` (which `monster` imports) wanted to import `conditions` for `refs.go`.

## Solution: Chains are an Event Concern

Chains are "just a type of publish" - they belong in `events/` not `combat/`. This breaks the cycle because `conditions` can import `events` without creating a cycle.

```
Before: conditions → combat (cycle when dnd5e → conditions)
After:  conditions → events (no cycle, events is lower-level)
```

---

## Files Created

### `rulebooks/dnd5e/events/chains.go`

Contains all chain-related types moved from `combat/`:

**Stage Constants:**
- `StageBase`
- `StageFeatures`
- `StageConditions`
- `StageEquipment`
- `StageFinal`
- `ModifierStages` slice defining execution order

**Damage Source Types:**
- `DamageSourceType` type
- `DamageSourceWeapon`
- `DamageSourceAbility`
- `DamageSourceRage`
- `DamageSourceSneakAttack`
- `DamageSourceDivineSmite`
- `DamageSourceElementalWeapon`
- `DamageSourceBrutalCritical`

**Structs:**
- `RerollEvent` - tracks die rerolls
- `DamageComponent` - with `Total()` method
- `AttackChainEvent`
- `DamageChainEvent`

**Typed Topics:**
- `AttackChain = events.DefineChainedTopic[AttackChainEvent]("dnd5e.combat.attack.chain")`
- `DamageChain = events.DefineChainedTopic[*DamageChainEvent]("dnd5e.combat.damage.chain")`

---

## Files Deleted

### `rulebooks/dnd5e/combat/stages.go`

Stages moved to `events/chains.go`

---

## Files Modified

### `combat/attack.go`

```go
// Add import
import dnd5eEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"

// Use dnd5eEvents.* instead of local types:
// - dnd5eEvents.DamageComponent
// - dnd5eEvents.DamageChainEvent
// - dnd5eEvents.ModifierStages
// - dnd5eEvents.StageFeatures
// - dnd5eEvents.DamageSourceWeapon
// - dnd5eEvents.DamageSourceAbility
// - dnd5eEvents.AttackChain
// - dnd5eEvents.DamageChain
```

### `conditions/raging.go`

```go
// Remove combat import
// Use:
// - dnd5eEvents.DamageChain
// - dnd5eEvents.DamageChainEvent
// - dnd5eEvents.DamageComponent
// - dnd5eEvents.DamageSourceRage
// - dnd5eEvents.StageFeatures
```

### `conditions/fighting_style.go`

```go
// Remove combat import
// Use:
// - dnd5eEvents.AttackChain
// - dnd5eEvents.AttackChainEvent
// - dnd5eEvents.DamageChain
// - dnd5eEvents.DamageChainEvent
// - dnd5eEvents.DamageComponent
// - dnd5eEvents.DamageSourceWeapon
// - dnd5eEvents.RerollEvent
// - dnd5eEvents.StageFeatures
```

### `conditions/brutal_critical.go`

```go
// Remove combat import
// Use:
// - dnd5eEvents.DamageChain
// - dnd5eEvents.DamageChainEvent
// - dnd5eEvents.DamageComponent
// - dnd5eEvents.DamageSourceBrutalCritical
// - dnd5eEvents.StageFeatures
```

### Test Files

**`conditions/raging_test.go`**, **`conditions/fighting_style_test.go`**, **`conditions/brutal_critical_test.go`**, **`combat/breakdown_test.go`**:

- Remove `combat` import (where applicable)
- Use `dnd5eEvents.*` for all chain types and damage source constants

---

## Also in This Commit: Unified Grant System

### `backgrounds/backgrounds.go`

```go
// Before
type Background string
func (b Background) Name() string { ... }

// After
type Background = core.ID
func Name(b Background) string { ... }
```

Converted methods to free functions:
- `Name(b Background) string`
- `Description(b Background) string`
- `IsVariant(b Background) bool`
- `BaseBackground(b Background) Background`

### `races/races.go`

```go
// Before
type Race string
func (r Race) Name() string { ... }

// After
type Race = core.ID
func Name(r Race) string { ... }
```

Converted methods to free functions:
- `Name(r Race) string`
- `Description(r Race) string`
- `IsSubrace(r Race) bool`
- `ParentRace(r Race) Race`

### `refs.go`

Uses canonical `Type` constants from domain packages:
- `features.Type`
- `conditions.Type`
- `skills.Type`
- `spells.Type`
- `races.Type`
- `backgrounds.Type`
- `classes.Type`

---

## Key Insight

The chain system (`AttackChain`, `DamageChain`) is part of the **event infrastructure**, not combat logic. Combat *uses* chains to collect modifiers, but the chain definitions themselves are event plumbing.

Moving them to `events/` makes the dependency graph cleaner and allows the `dnd5e` package to freely import domain packages like `conditions`, `features`, etc. for building refs.
