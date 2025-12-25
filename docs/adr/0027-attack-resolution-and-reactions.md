# ADR-0027: Attack Resolution and Reactions

Date: 2024-12-25

## Status

Proposed

## Context

ADR-0026 established how damage flows through the event system. But attacks involve more than just damage - they require resolving modifiers (advantage, attack bonuses) before rolling, and reactions can interrupt at various points.

**Key challenges:**
- Sneak Attack needs to know about advantage BEFORE the roll to determine eligibility
- Shield spell triggers AFTER the roll but BEFORE hit determination, modifying AC
- Uncanny Dodge triggers AFTER hit determination but BEFORE damage
- Attack of Opportunity triggers on movement, consuming the reaction resource

We need a consistent model for:
1. Multi-phase attack resolution
2. Reaction windows at appropriate points
3. Reaction resource consumption

## Decision

**Attack resolution uses a three-phase event model with reaction windows between phases.**

### The Full Attack Flow

```
┌─────────────────────────────────────────────────────────────────────┐
│  PHASE 0: DECLARE (optional reactions before anything happens)     │
│                                                                     │
│  AttackDeclared event published                                     │
│       ↓                                                             │
│  [Reaction Window]                                                  │
│       - Sentinel: "Enemy attacked my ally, I attack them first"    │
│       - Protection: "Ally attacked, I impose disadvantage"         │
│       - Consumes reactor's reaction                                 │
└─────────────────────────────────────────────────────────────────────┘
                              ↓
┌─────────────────────────────────────────────────────────────────────┐
│  PHASE 1: RESOLVE ATTACK (gather modifiers before rolling)         │
│                                                                     │
│  ResolveAttack chain published                                      │
│       ↓                                                             │
│  [Stage: base] → attacker's base attack bonus                       │
│       ↓                                                             │
│  [Stage: features] → Sneak Attack marks eligibility                 │
│       ↓                                                             │
│  [Stage: conditions] → Poisoned? Restrained?                        │
│       ↓                                                             │
│  [Stage: equipment] → Magic weapon bonus                            │
│       ↓                                                             │
│  [Stage: situational] → Flanking advantage, cover penalties         │
│       ↓                                                             │
│  Chain.Execute() → returns ResolvedAttack                           │
│       - HasAdvantage, HasDisadvantage                               │
│       - AttackBonus                                                 │
│       - SneakAttackEligible                                         │
└─────────────────────────────────────────────────────────────────────┘
                              ↓
┌─────────────────────────────────────────────────────────────────────┐
│  PHASE 2: ROLL AND EVALUATE                                         │
│                                                                     │
│  Roll d20 (with advantage/disadvantage)                             │
│       ↓                                                             │
│  Calculate total: roll + attack bonus                               │
│       ↓                                                             │
│  AttackRolled event published (contains roll result, target AC)     │
│       ↓                                                             │
│  [Reaction Window - CAN CHANGE OUTCOME]                             │
│       - Shield: "Add +5 to my AC"                                   │
│       - Cutting Words: "Subtract d6 from attack roll"               │
│       - Lucky: "Reroll that die"                                    │
│       - Consumes reactor's reaction (and spell slot if applicable)  │
│       ↓                                                             │
│  Determine hit/miss/crit with modified values                       │
│       ↓                                                             │
│  AttackResolved event published                                     │
│       - Hit: bool                                                   │
│       - Critical: bool                                              │
│       - AttackRoll, FinalAC                                         │
└─────────────────────────────────────────────────────────────────────┘
                              ↓ (if hit)
┌─────────────────────────────────────────────────────────────────────┐
│  PHASE 3: RESOLVE DAMAGE (see ADR-0026)                             │
│                                                                     │
│  [Reaction Window - BEFORE damage resolution]                       │
│       - Uncanny Dodge: "Halve incoming damage"                      │
│       - Consumes reactor's reaction                                 │
│       ↓                                                             │
│  ResolveDamage chain published                                      │
│       - Sneak Attack adds dice (if eligible AND hit)                │
│       - Rage adds flat bonus                                        │
│       - Critical doubles dice                                       │
│       ↓                                                             │
│  ApplyDamage (character mutated in gamectx)                         │
│       ↓                                                             │
│  DamageApplied notification                                         │
│       ↓                                                             │
│  [Reaction Window - AFTER damage]                                   │
│       - Hellish Rebuke: "You hurt me, take fire damage"             │
│       - Consumes reactor's reaction + spell slot                    │
└─────────────────────────────────────────────────────────────────────┘
```

### Reaction Resource

Reactions are tracked as a `RecoverableResource`:

```go
type ReactionResource struct {
    Available bool
    ResetsOn  ResetTrigger // TurnStart
}

// Check and consume atomically
func (r *ReactionResource) TryUse() bool {
    if !r.Available {
        return false
    }
    r.Available = false
    return true
}
```

Features that use reactions check availability before triggering:

```go
// Shield spell subscribes to AttackRolled
AttackRolled.On(bus).Subscribe(func(ctx context.Context, event *AttackRolledEvent) {
    if event.TargetID != s.CasterID {
        return
    }

    // Would this hit without Shield?
    if event.AttackTotal < event.TargetAC {
        return // Already a miss, don't waste reaction
    }

    // Would Shield make it miss?
    if event.AttackTotal >= event.TargetAC + 5 {
        return // Still hits even with +5 AC, don't waste it
    }

    // Check reaction available
    caster, _ := gamectx.GetCharacter(ctx, s.CasterID)
    if !caster.Reaction.TryUse() {
        return // Already used reaction this round
    }

    // Check spell slot
    if !caster.Resources.TryConsume("spell_slot_1", 1) {
        return // No spell slots
    }

    // Modify the event - add AC bonus
    event.TargetAC += 5
})
```

### Attack of Opportunity

AoO triggers on movement, not attacks. It subscribes to movement events:

```go
// Entity movement is published step-by-step
MovementStep.On(bus).Subscribe(func(ctx context.Context, event *MovementStepEvent) {
    // Did entity leave a threatened square?
    if !s.ThreatenedSquares.Contains(event.FromPosition) {
        return
    }
    if s.ThreatenedSquares.Contains(event.ToPosition) {
        return // Still in threatened area
    }

    // Check reaction available
    threatener, _ := gamectx.GetCharacter(ctx, s.CharacterID)
    if !threatener.Reaction.TryUse() {
        return
    }

    // Make the attack - goes through full attack flow
    combat.ResolveAttack(ctx, bus, &ResolveAttackInput{
        AttackerID: s.CharacterID,
        TargetID:   event.EntityID,
        IsAoO:      true, // Sentinel checks this
    })
})
```

**Sentinel feat** modifies this:
- Subscribes to AttackResolved for AoO attacks
- If AoO hits, sets target's speed to 0 (event modifier)

### Key Design Elements

1. **`combat.ResolveAttack(ctx, bus, input)`** - Orchestrates phases 1-2
2. **`combat.DealDamage(ctx, bus, input)`** - Orchestrates phase 3 (ADR-0026)
3. **Reaction windows are events** - Not special infrastructure
4. **Events are mutable during chain** - Reactions modify AC, roll, etc.
5. **Reaction consumption is atomic** - TryUse returns false if already used

### Event Types

```go
// Phase 0
type AttackDeclaredEvent struct {
    AttackerID   string
    TargetID     string
    WeaponID     string
    AttackType   AttackType // Melee, Ranged, Spell
}

// Phase 1 (chain)
type ResolveAttackEvent struct {
    AttackerID        string
    TargetID          string
    AttackBonus       int
    HasAdvantage      bool
    HasDisadvantage   bool
    SneakAttackEligible bool
    // ... modifiers add to these
}

// Phase 2
type AttackRolledEvent struct {
    AttackerID   string
    TargetID     string
    NaturalRoll  int      // The d20 result
    AttackTotal  int      // Roll + modifiers
    TargetAC     int      // Can be modified by reactions
    IsCritical   bool
}

type AttackResolvedEvent struct {
    AttackerID   string
    TargetID     string
    Hit          bool
    Critical     bool
    AttackRoll   int
    FinalAC      int
}

// Phase 3 - see ADR-0026
```

## Consequences

### Positive

- **Clear phases** - Each step has defined inputs/outputs
- **Reactions are just subscribers** - No special reaction infrastructure
- **Composable** - Shield, Cutting Words, Lucky all work the same way
- **Sneak Attack works** - Eligibility determined before roll, damage added after hit
- **AoO is a real attack** - Goes through full flow, can crit, triggers reactions

### Negative

- **Multiple events per attack** - More ceremony than simple "roll to hit"
- **Mutable events** - Must be careful about modification order
- **Movement needs step-by-step events** - Can't just teleport entities

### Neutral

- **Reaction timing is explicit** - Each window is a distinct event
- **Some reactions are "automatic"** - Shield, Uncanny Dodge fire without player input in tactical AI

## Alternatives Considered

### A: Single ResolveAttack event with all phases

One big event that goes through all stages.

**Rejected because:**
- Reactions need to trigger at specific moments
- Shield can't modify AC after damage is already calculated
- Loses clarity about what happens when

### B: Reactions as interceptors, not subscribers

Special "interceptor" pattern that can halt/modify flow.

**Rejected because:**
- Unnecessary complexity - events can be mutable
- Subscribers already have the power to modify events
- Consistency with rest of event system

### C: No step-by-step movement

Movement is atomic - entity teleports from A to B.

**Rejected because:**
- AoO requires knowing when entity leaves threatened squares
- Difficult terrain, traps need step-by-step
- Movement already complex (dash, difficult terrain, flying)

## Example: Rogue Sneak Attack Flow

```go
// 1. Rogue attacks goblin
combat.ResolveAttack(ctx, bus, &ResolveAttackInput{
    AttackerID: rogueID,
    TargetID:   goblinID,
    WeaponID:   daggerID,
})

// 2. ResolveAttack chain executes
//    - SneakAttack subscriber checks: ally adjacent to goblin? YES
//    - Marks event.SneakAttackEligible = true

// 3. Roll happens: natural 18 + 7 = 25 vs AC 13 = HIT

// 4. AttackResolved published with Hit=true, Critical=false

// 5. DealDamage called (since hit)
combat.DealDamage(ctx, bus, &DealDamageInput{
    AttackerID: rogueID,
    TargetID:   goblinID,
    Source:     DamageSourceAttack,
    Instances:  []DamageInstance{{Type: Piercing, Dice: "1d4+4"}},
    Context: &AttackContext{
        SneakAttackEligible: true, // Passed from resolve phase
        Critical:            false,
    },
})

// 6. ResolveDamage chain executes
//    - SneakAttack subscriber sees eligible=true, adds 2d6
//    - Final instances: [{Piercing, "1d4+4"}, {Piercing, "2d6"}]

// 7. Damage applied, goblin HP reduced
```

## Example: Shield Spell Reaction

```go
// 1. Goblin attacks wizard, rolls 15 + 4 = 19

// 2. AttackRolled published: {AttackTotal: 19, TargetAC: 14}

// 3. Shield subscriber fires:
//    - 19 >= 14, would hit
//    - 19 < 14 + 5 = 19, Shield would make it miss!
//    - wizard.Reaction.TryUse() = true
//    - wizard.Resources.TryConsume("spell_slot_1", 1) = true
//    - event.TargetAC += 5 → now 19

// 4. Hit determination: 19 >= 19? NO (ties go to defender with Shield's wording)
//    Actually in 5e ties go to attacker, so 19 >= 19 = hit
//    Let me reconsider... Shield says AC becomes 19, attack is 19, that's a hit
//    Wizard might choose not to cast it! Need decision logic.

// Better: Shield subscriber checks if it would GUARANTEE a miss
func shouldCastShield(attackTotal, currentAC int) bool {
    return attackTotal >= currentAC && attackTotal < currentAC + 5
}
```

## Related ADRs

- **ADR-0024**: ChainedTopic event system
- **ADR-0025**: gamectx for entity lookup
- **ADR-0026**: Damage application via event chain
