# Spell Architecture Brainstorm

**Date:** 2024-12-03
**Status:** Draft - Capturing discussion for future implementation

## Overview

This document captures our thinking about how spells should integrate with the existing toolkit patterns. The goal is to establish patterns that work for three representative spell types before implementing the full spell system.

## Three Spell Categories

| Category | Example | Pattern |
|----------|---------|---------|
| **Attack Spells** | Fire Bolt | Spell attack through AttackChain → DamageChain |
| **Heal Spells** | Cure Wounds | HealChain (new) or direct event |
| **Condition Spells** | Bless | Action activates → Condition subscribes to chains |

## Spells as Actions

Spells fit the `core.Action[T]` pattern:

```go
type SpellInput struct {
    SlotLevel      int           // 0 for cantrips
    TargetIDs      []string      // For targeted spells
    TargetPosition *Position     // For AoE spells
    // Slot consumption handled by caller
}

type FireBolt struct {
    CasterID string
    // Cantrip - no slot needed
}

func (f *FireBolt) CanActivate(ctx context.Context, owner core.Entity, input SpellInput) error {
    // Check line of sight, range, etc.
    return nil
}

func (f *FireBolt) Activate(ctx context.Context, owner core.Entity, input SpellInput) error {
    // Roll spell attack through AttackChain
    // Roll damage through DamageChain
    return nil
}
```

## Condition-Applying Spells

Same pattern as Rage ability → Raging condition:

```
Shield Spell (Action) → Activates → Shielded Condition → Subscribes to AC/hit events
Bless Spell (Action)  → Activates → Blessed Condition  → Subscribes to attack/save chains
Rage Ability (Action) → Activates → Raging Condition   → Subscribes to damage chain
```

The spell is the **trigger**, the condition is the **persistent effect**:

```go
func (s *ShieldSpell) Activate(ctx context.Context, owner core.Entity, input SpellInput) error {
    shielded := &ShieldedCondition{
        CasterID:  owner.GetID(),
        ACBonus:   5,
        ExpiresAt: "start_of_next_turn",
    }
    return conditionManager.Apply(ctx, shielded)
}
```

## Can Spell Attacks Use Existing Chains?

### AttackChainEvent - Already Generic

```go
type AttackChainEvent struct {
    AttackerID      string  // works for caster
    TargetID        string  // works
    AttackRoll      int     // d20 roll - same for spells
    AttackBonus     int     // spell attack bonus goes here
    TargetAC        int     // same
    IsNaturalTwenty bool    // same
    IsNaturalOne    bool    // same
}
```

**Verdict:** Works for spell attacks as-is. Bless adds +1d4 to "attack rolls" - doesn't care if sword or Fire Bolt.

### DamageChainEvent - Has Weapon Coupling

```go
type DamageChainEvent struct {
    AttackerID   string             // generic
    TargetID     string             // generic
    Components   []DamageComponent  // generic
    DamageType   string             // generic - "fire", "radiant"
    IsCritical   bool               // generic
    WeaponDamage string             // WEAPON-SPECIFIC naming
    AbilityUsed  abilities.Ability  // could be INT/WIS/CHA for spells
}
```

**Issue:** `WeaponDamage` field name assumes weapon.

### Proposed Change: Rename to BaseDamage

```go
type DamageChainEvent struct {
    // ...
    BaseDamage  string  // was WeaponDamage - "1d8", "8d6", etc.
    // ...
}
```

This is just "what dice started the damage" - works for weapons or spells.

### Add Spell Damage Source

```go
const (
    DamageSourceWeapon DamageSourceType = "weapon"
    DamageSourceSpell  DamageSourceType = "spell"  // NEW
    // ...
)
```

## Heal Chain Consideration

Do we need a `HealChain` for modifiers like Life Cleric's Disciple of Life (+2 + spell level)?

**Option A: HealChain (parallel to DamageChain)**
```go
type HealChainEvent struct {
    HealerID    string
    TargetID    string
    BaseHealing string  // "1d8"
    SpellLevel  int
    Components  []HealComponent
}

var HealChain = events.DefineChainedTopic[*HealChainEvent]("dnd5e.combat.heal.chain")
```

**Option B: Direct event, no chain**
```go
// Simple spells just publish healing
healTopic.Publish(ctx, HealEvent{
    TargetID: target.GetID(),
    Amount:   rolledHealing,
    Source:   "cure_wounds",
})
```

**Decision needed:** Do enough things modify healing to justify a chain? Life Cleric, Beacon of Hope, etc.

## Resource Management (Parked)

Spell slots, ki points, rage charges, Channel Divinity - all "limited resources" with different recovery rules.

For now:
- `SpellInput.SlotLevel` indicates the slot used
- Slot consumption/validation happens before `Activate` is called
- Actual resource system is a separate concern

## Concentration (Parked)

Concentration spells need:
- Only one concentration spell at a time
- Constitution saves when taking damage
- Condition removal on failed save or voluntary end

This could be:
- A condition on the caster that tracks the spell
- A flag on the spell condition itself
- A separate concentration manager

**Park for now** - basic spell patterns first.

## Implementation Scope

The issue should focus on proving the patterns with three spells:

### 1. Fire Bolt (Attack Cantrip)
- Uses `AttackChain` with spell attack bonus
- Uses `DamageChain` with `DamageSourceSpell`
- Validates spell attacks work through existing chains
- Automatically gets Bless/Bane modifiers

### 2. Cure Wounds (Heal Spell)
- Decides: HealChain or direct event?
- Takes `SpellInput.SlotLevel` for upcast (1d8 per level)
- Touch range targeting

### 3. Bless (Condition Spell)
- Action that applies `BlessedCondition` to targets
- `BlessedCondition` subscribes to attack and save chains
- Adds 1d4 modifier
- Concentration (basic - just track that it's active)

## Files to Modify

### Existing Files
- `rulebooks/dnd5e/combat/attack.go`
  - Rename `WeaponDamage` → `BaseDamage`
  - Add `DamageSourceSpell` constant

### New Files (in rulebooks/dnd5e/)
- `spells/spell_input.go` - SpellInput type
- `spells/fire_bolt.go` - Attack spell implementation
- `spells/cure_wounds.go` - Heal spell implementation
- `spells/bless.go` - Condition spell action
- `conditions/blessed.go` - Blessed condition

## Open Questions

1. **HealChain** - Do we need it? What modifies healing?
2. **SpellInput shape** - What fields are needed?
3. **Existing `mechanics/spells/`** - Use it or build fresh in rulebook?
4. **Saving throws** - How do spells that require saves work?
5. **AoE** - How does Fireball hit multiple targets?

## Relationship to Existing Code

There's already `mechanics/spells/` with a `Spell` interface and `CastContext`. We should:
1. Review if that fits our patterns
2. Decide if we extend it or build fresh in `rulebooks/dnd5e/spells/`
3. Keep learning in the rulebook before moving to mechanics

## Next Steps

1. Create GitHub issue capturing this scope
2. Start with Fire Bolt to validate chain integration
3. Add Cure Wounds to establish heal pattern
4. Add Bless to establish condition-spell pattern
5. Document learnings for future spell implementation

## References

- `rulebooks/dnd5e/combat/attack.go` - Current attack/damage chains
- `rulebooks/dnd5e/conditions/fighting_style.go` - Chain subscription pattern
- `rulebooks/dnd5e/features/rage.go` - Action → Condition pattern
- `mechanics/spells/` - Existing spell infrastructure
- Issue #382 - gamectx pattern (related context work)
