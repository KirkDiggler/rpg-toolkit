# ADR-0029: Chain-Based Combat Modifiers

Date: 2025-01-09

## Status

Proposed

## Context

Conditions like Dodging and Disengaging need to modify combat mechanics:
- Dodging: Disadvantage on attacks against you, advantage on DEX saves
- Disengaging: Prevents opportunity attacks from your movement

We need patterns that:
1. Allow conditions to hook into attacks, saves, and movement
2. Scale without explosion of boolean flags
3. Provide clear context about why things are happening
4. Work consistently across different game mechanics

ADR-0027 established attack resolution phases. This ADR extends that pattern to saving throws and movement, and establishes conventions for modifier organization.

## Decision

### 1. Type Enums Over Booleans for Mutually Exclusive States

When something can only be one type, use an enum instead of booleans:

```go
// GOOD: Attack can only be one type
type AttackType string

const (
    AttackTypeStandard    AttackType = "standard"
    AttackTypeOpportunity AttackType = "opportunity"
)

// BAD: Boolean flags
type AttackChainEvent struct {
    IsOpportunityAttack bool  // What if we add more types?
    IsReactionAttack    bool  // Explosion of booleans
}
```

This applies to any mutually exclusive categorization.

### 2. Separate Slices Per Modifier Category

Each modifier category gets its own slice for O(1) existence checks:

```go
type AttackChainEvent struct {
    AttackType          AttackType
    AdvantageSources    []AttackModifierSource  // Sources granting advantage
    DisadvantageSources []AttackModifierSource  // Sources imposing disadvantage
    CancellationSources []AttackModifierSource  // Sources cancelling the attack
}

// O(1) check
if len(event.CancellationSources) > 0 {
    // Attack is cancelled
}
```

Alternative considered: Single `Modifiers []Modifier` slice with type field.
Rejected because: Requires O(n) iteration to find specific modifier types.

### 3. SavingThrowChain for Save Modifiers

New chain allows conditions to modify saving throws:

```go
type SavingThrowChainEvent struct {
    SaverID string
    Ability abilities.Ability
    DC      int

    Cause SaveCause

    AdvantageSources    []SaveModifierSource
    DisadvantageSources []SaveModifierSource
    BonusSources        []SaveBonusSource
}

var SavingThrowChain = events.DefineChainedTopic[*SavingThrowChainEvent](
    "dnd5e.saves.chain")
```

### 4. SaveCause Structure for Context

Bundles the "why/what/who" of a saving throw:

```go
type SaveTrigger string

const (
    SaveTriggerSpell         SaveTrigger = "spell"
    SaveTriggerTrap          SaveTrigger = "trap"
    SaveTriggerConcentration SaveTrigger = "concentration"
    SaveTriggerFeature       SaveTrigger = "feature"
    SaveTriggerEnvironment   SaveTrigger = "environment"
)

type SaveCause struct {
    Trigger        SaveTrigger     // Why: concentration, spell, trap
    EffectRef      *core.Ref       // What: refs.Spells.Fireball()
    InstigatorID   string          // Who: "goblin-1"
    InstigatorType core.EntityType // What kind: "monster", "character"
}
```

This enables features like:
- War Caster: `if cause.Trigger == SaveTriggerConcentration`
- Magic Resistance: `if cause.Trigger == SaveTriggerSpell`

The `InstigatorType` allows the game server to know which list to look up the entity from (monsters, characters, traps, etc.).

### 5. MovementChain for Path-Based Movement

Movement flows through a chain with the full path, enabling:
- Disengaging to prevent opportunity attacks
- Traps to halt movement
- Sentinel to stop movement on OA hit

```go
type MovementChainEvent struct {
    EntityID string
    Path     []Position  // Full path including start and destination

    // Populated by movement resolver
    ThreatenedByAtStep map[int][]string  // step index -> threatening entity IDs

    // Modified by conditions
    OAPreventionSources []MovementModifierSource  // Disengaging adds here

    // Set by resolver based on OA results, traps, etc.
    MovementStopped bool
    StopAtIndex     int
}

var MovementChain = events.DefineChainedTopic[*MovementChainEvent](
    "dnd5e.combat.movement.chain")
```

Movement resolution flow:
1. UI sends path
2. MovementChain fires with full path
3. Conditions modify (Disengaging adds to OAPreventionSources)
4. Resolver processes step-by-step:
   - Check threats at each step
   - If leaving reach and no prevention sources, fire AttackChain for OA
   - If OA hits, check for Sentinel (stops movement)
   - If trap, resolve trap effect
5. Publish individual events as they happen (streaming to UI)
6. Publish MovementResolvedEvent with final position

### 6. Combat Abilities Apply Conditions Directly

Combat abilities (Dodge, Disengage) create and apply conditions directly, not via events:

```go
// In Dodge ability's Activate method:
func (d *DodgeAbility) Activate(ctx context.Context, owner core.Entity, input AbilityInput) error {
    condition := conditions.NewDodgingCondition(owner.GetID())
    if err := condition.Apply(ctx, input.Bus); err != nil {
        return err
    }
    // Store reference for later removal if needed
    return nil
}
```

This is cleaner than publishing DodgeActivatedEvent and having the condition listen for it.

### 7. Condition Lifecycle

Conditions subscribe to relevant events in their `Apply` method:

**DodgingCondition:**
- Subscribes to AttackChain: adds disadvantage when targeted
- Subscribes to SavingThrowChain: adds advantage on DEX saves
- Subscribes to TurnStartEvent: removes self when owner's turn starts

**DisengagingCondition:**
- Subscribes to AttackChain: adds to CancellationSources for OA attacks
- Subscribes to TurnEndEvent: removes self when owner's turn ends

## Consequences

### Positive

- **O(1) modifier checks** - Separate slices enable fast existence checks
- **Type safety** - Enums prevent invalid combinations
- **Clear context** - SaveCause tells the full story of why a save is happening
- **Extensible** - New modifier types just add new slices
- **Consistent pattern** - Attacks, saves, and movement all use chain + modifier sources
- **Streaming feedback** - UI receives events as they happen during movement

### Negative

- **More event types** - Each mechanic needs its own chain event
- **Struct growth** - Adding modifier slices grows event structs
- **Path calculation required** - Movement needs full path upfront, not just destination

### Neutral

- **Conditions are ephemeral** - Applied by abilities, removed by turn events
- **Resolution logic in game server** - Chains collect modifiers, server makes decisions

## Related ADRs

- **ADR-0024**: Typed topics pattern
- **ADR-0026**: Damage application via event chain
- **ADR-0027**: Attack resolution and reactions
