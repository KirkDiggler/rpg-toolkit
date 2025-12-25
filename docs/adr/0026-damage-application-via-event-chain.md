# ADR-0026: Damage Application via Event Chain

Date: 2024-12-24

## Status

Proposed

## Context

While developing the rpg-api encounter system, we discovered an architectural problem: damage/HP tracking was split between the API and toolkit inconsistently.

**The problem:**
- API maintained `enc.CharacterHP` map tracking combat HP separately from character data
- Monster attacks updated this map but never persisted to character repo
- Player attacks did persist to character repo
- Two sources of truth led to desync bugs

**The deeper question:** Where should damage application live?

The API layer was doing too much - loading characters, calculating damage, applying HP changes, persisting. This made it difficult for conditions (poison, ongoing fire) to apply damage, and prevented proper modifier composition (resistance, vulnerability, rage bonus).

## Decision

**Damage application lives in the toolkit, using a two-phase event model: Resolve (gather modifiers) → Apply (mutate character) → Notify (inform subscribers).**

### Two-Phase Flow

```
┌─────────────────────────────────────────────────────────────┐
│  RESOLVE PHASE (chain gathers modifiers)                    │
│                                                             │
│  ResolveDamage chain published                              │
│       ↓                                                     │
│  [Stage: base] → raw damage instances                       │
│       ↓                                                     │
│  [Stage: features] → Rage adds +2 to melee                  │
│       ↓                                                     │
│  [Stage: conditions] → Poisoned? Weakened?                  │
│       ↓                                                     │
│  [Stage: equipment] → Resistance to fire halves fire dmg    │
│       ↓                                                     │
│  [Stage: final] → any last adjustments                      │
│       ↓                                                     │
│  Chain.Execute() → returns final resolved damage            │
└─────────────────────────────────────────────────────────────┘
                            ↓
        DealDamage() gets final result back
                            ↓
        Publisher calls target.ApplyDamage() directly
                            ↓
        Character in gamectx is mutated (HP reduced)
                            ↓
┌─────────────────────────────────────────────────────────────┐
│  NOTIFY PHASE (informational, no modification)              │
│                                                             │
│  DamageApplied event published                              │
│       ↓                                                     │
│  [Subscriber: Concentration] → "I took 15 damage, DC 10"    │
│  [Subscriber: Combat Log] → "Goblin hit you for 15"         │
│  [Subscriber: Death Watch] → "HP now 0, start death saves"  │
└─────────────────────────────────────────────────────────────┘
```

**Key insight: Publisher applies damage, not a subscriber.** This ensures:
- Damage is guaranteed to apply (no silent failures if nothing subscribed)
- `DealDamage` can return useful output (damage dealt, target dropped to 0?)
- Clear control flow - DealDamage orchestrates everything

### Event Naming

- **`ResolveDamage`** (chain) - Subscribers MODIFY the damage (add resistance, rage bonus, etc.)
- **`DamageApplied`** (notification) - Subscribers REACT to what happened (concentration checks, logging)

### Key Design Elements

1. **`combat.DealDamage(ctx, bus, input)`** - Central function that orchestrates the flow
2. **`ApplyDamageInput`** - Follows Input/Output pattern for extensibility
3. **gamectx returns errors, not panics** - `GetCharacter()` instead of `RequireCharacter()`
4. **API loads context, toolkit handles logic** - API calls `gamectx.WithCharacters()`, then toolkit functions
5. **Characters in gamectx are mutated** - API picks them up after combat to persist

### For Different Damage Sources

**Conditions are publishers, not damage handlers.** They subscribe to trigger events and publish damage through the same `DealDamage` flow.

- **Single target (attack)**: Attacker calls `DealDamage`, target resolved via gamectx
- **AoE (fireball)**: Spell calls `DealDamage` per target after save resolution
- **DoT (poison)**: Condition subscribes to `TurnStart`, then calls `DealDamage` targeting owner

```go
// Poison condition subscribes to its TRIGGER, then PUBLISHES damage
TurnStart.On(bus).Subscribe(func(ctx context.Context, event TurnStartEvent) {
    if event.EntityID != p.OwnerID {
        return
    }

    // Poison calls DealDamage just like any attack would
    combat.DealDamage(ctx, bus, &combat.DealDamageInput{
        TargetID:  p.OwnerID,
        Source:    DamageSourceCondition,
        Instances: []DamageInstance{{Type: Poison, Dice: "1d6"}},
    })
})
```

## Consequences

### Positive

- **Single source of truth**: Character owns its HP, no duplicate tracking
- **Composable modifiers**: Resistance, rage, vulnerability all participate via subscriptions
- **Conditions can deal damage**: Poison tick just publishes to the chain like any other source
- **API stays simple**: Load entities, call toolkit function, persist results
- **Consistent with existing patterns**: Uses ChainedTopic (ADR-0024), gamectx (ADR-0025)

### Negative

- **Migration required**: Existing rpg-api code needs refactoring
- **More ceremony**: Simple "deal 5 damage" requires going through the chain
- **Context setup**: API must load all relevant entities into gamectx before combat

### Neutral

- **Healing uses same pattern**: HealingChain with stages
- **Death saves tracked as RecoverableResource**: Fits existing resource pattern
- **Massive damage rule**: Room in ApplyDamage for future implementation

## Alternatives Considered

### A: Keep damage in API layer

API calculates and applies damage directly.

**Rejected because:**
- Violates "toolkit is smart, API is dumb" principle
- Conditions can't easily deal damage (poison, ongoing effects)
- No centralized place for modifier composition

### B: Damage events carry full character objects

DamageChainEvent includes the full Character being damaged.

**Rejected because:**
- Violates "events carry actions, not objects" (Journey 044)
- Creates coupling between event publishers and Character internals
- Potential for stale data if character changes mid-chain

### C: No event system, direct method calls

`attacker.Attack(defender)` with no events.

**Rejected because:**
- Can't compose modifiers (resistance + rage + bless + etc.)
- Conditions can't participate in damage modification
- Loses the flexibility of the event-driven architecture

## Example

```go
// API layer - simple orchestration
func (o *Orchestrator) executeMonsterAttack(ctx context.Context, monsterID, targetID string) error {
    // 1. Load everything into gamectx (API's job)
    ctx = gamectx.WithCharacters(ctx, allCharacters)
    ctx = gamectx.WithMonsters(ctx, allMonsters)

    // 2. Resolve attack (toolkit handles modifiers, hit/miss)
    attackResult, err := combat.ResolveAttack(ctx, o.bus, attackInput)
    if err != nil {
        return err
    }

    // 3. If hit, deal damage (toolkit handles resistance, applies HP)
    if attackResult.Hit {
        _, err = combat.DealDamage(ctx, o.bus, &combat.DealDamageInput{
            TargetID:  targetID,
            Source:    combat.DamageSourceAttack,
            Instances: attackResult.DamageInstances,
        })
        if err != nil {
            return err
        }
    }

    // 4. Characters in gamectx are already mutated, just persist
    for _, char := range gamectx.GetAllCharacters(ctx) {
        o.charRepo.Update(ctx, char.ToData())
    }

    return nil
}
```

```go
// Toolkit - DealDamage orchestrates the two-phase flow
func DealDamage(ctx context.Context, bus events.EventBus, input *DealDamageInput) (*DealDamageOutput, error) {
    // Build initial event with all damage instances
    event := &ResolveDamageEvent{
        TargetID:  input.TargetID,
        Source:    input.Source,
        Instances: input.Instances,
    }

    // PHASE 1: RESOLVE - gather modifiers via chain
    chain := events.NewStagedChain[*ResolveDamageEvent](ModifierStages)
    resolveChain := dnd5eEvents.ResolveDamage.On(bus)
    modifiedChain, err := resolveChain.PublishWithChain(ctx, event, chain)
    if err != nil {
        return nil, err
    }

    // Execute chain in stage order (base → features → conditions → equipment → final)
    finalEvent, err := modifiedChain.Execute(ctx, event)
    if err != nil {
        return nil, err
    }

    // PHASE 2: APPLY - publisher applies damage directly (not a subscriber)
    target, err := gamectx.GetCharacter(ctx, input.TargetID)
    if err != nil {
        // Maybe it's a monster?
        target, err = gamectx.GetMonster(ctx, input.TargetID)
        if err != nil {
            return nil, fmt.Errorf("target %s not found in gamectx", input.TargetID)
        }
    }

    // Apply each damage instance (fire + slashing = 2 instances, resistance per type)
    var totalDamage int
    for _, instance := range finalEvent.Instances {
        totalDamage += instance.Amount
    }

    result := target.ApplyDamage(ctx, &ApplyDamageInput{
        Instances: finalEvent.Instances,
        Total:     totalDamage,
    })

    // PHASE 3: NOTIFY - inform subscribers (concentration, logging, death checks)
    dnd5eEvents.DamageApplied.On(bus).Publish(ctx, &DamageAppliedEvent{
        TargetID:     input.TargetID,
        Source:       input.Source,
        TotalDamage:  totalDamage,
        CurrentHP:    result.CurrentHP,
        DroppedToZero: result.DroppedToZero,
    })

    return &DealDamageOutput{
        TotalDamage:   totalDamage,
        CurrentHP:     result.CurrentHP,
        DroppedToZero: result.DroppedToZero,
    }, nil
}
```

```go
// Rage feature subscribes to ResolveDamage chain at features stage
func (r *RageFeature) Subscribe(bus events.EventBus) {
    dnd5eEvents.ResolveDamage.On(bus).SubscribeWithStage(
        ModifierStageFeatures,
        func(ctx context.Context, event *ResolveDamageEvent) {
            // Only modify melee weapon damage from this character
            if event.Source != combat.DamageSourceAttack {
                return
            }
            attacker, _ := gamectx.GetCharacter(ctx, event.AttackerID)
            if attacker.ID != r.CharacterID || !r.IsActive {
                return
            }

            // Add rage bonus to each damage instance
            for i := range event.Instances {
                event.Instances[i].Amount += r.DamageBonus
            }
        },
    )
}
```

```go
// Concentration subscribes to DamageApplied notification
func (c *ConcentrationTracker) Subscribe(bus events.EventBus) {
    dnd5eEvents.DamageApplied.On(bus).Subscribe(
        func(ctx context.Context, event *DamageAppliedEvent) {
            if event.TargetID != c.CharacterID || !c.IsConcentrating {
                return
            }

            // DC = 10 or half damage, whichever is higher
            dc := max(10, event.TotalDamage/2)
            // Trigger concentration save...
        },
    )
}
```
