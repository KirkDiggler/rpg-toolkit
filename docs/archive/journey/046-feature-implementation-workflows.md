# Journey 046: Feature Implementation Workflows

## The Context

We've proven the event bus architecture works with the Rage feature. A barbarian can rage, the raging condition subscribes to the damage chain, and we see the +2 damage bonus appearing in the combat breakdown. The pattern is validated.

Now we need to systematically implement class features. This document establishes workflows for adding features to the toolkit, using Rage as the proven template.

## The Architecture (What We Proved Works)

```
┌─────────────────────────────────────────────────────────────────────┐
│                        EVENT BUS                                     │
├─────────────────────────────────────────────────────────────────────┤
│                                                                      │
│  TypedTopics (notifications)      ChainedTopics (modifiers)         │
│  ├── AttackTopic                  ├── AttackChain                   │
│  ├── DamageReceivedTopic          └── DamageChain                   │
│  ├── TurnStartTopic                                                 │
│  ├── TurnEndTopic                                                   │
│  ├── ConditionAppliedTopic                                          │
│  └── ConditionRemovedTopic                                          │
│                                                                      │
└─────────────────────────────────────────────────────────────────────┘

Feature.Activate()
    └── Publishes ConditionAppliedEvent
        └── Condition.Apply(bus) subscribes to:
            ├── TypedTopics (track state, end conditions)
            └── ChainedTopics (add modifiers at appropriate stage)
                    │
                    ▼
            ┌──────────────────────────────────────┐
            │       ModifierStages (order)         │
            │  1. StageBase       - dice, ability  │
            │  2. StageFeatures   - rage, sneak    │
            │  3. StageConditions - bless, bane    │
            │  4. StageEquipment  - magic items    │
            │  5. StageFinal      - resistance     │
            └──────────────────────────────────────┘
```

## The Decision Tree

When adding a new game mechanic, ask:

```
Does it MODIFY a roll or damage?
├── YES → Use a ChainedTopic (AttackChain or DamageChain)
│         └── Subscribe with SubscribeWithChain()
│             └── Add modifier at appropriate stage
│
└── NO → Does it need to TRACK state or TRIGGER on events?
         ├── YES → Use TypedTopics (Subscribe())
         │         └── Track turns, hits, etc.
         │
         └── NO → Is it a ONE-SHOT effect?
                  └── Just publish event, don't subscribe
```

## Workflow 1: Adding a Damage Modifier

**Examples:** Rage (+2 damage), Sneak Attack (+Xd6), Divine Smite (+2d8)

### Step 1: Add DamageSourceType constant
File: `rulebooks/dnd5e/combat/attack.go`

```go
const (
    DamageSourceWeapon      DamageSourceType = "weapon"
    DamageSourceAbility     DamageSourceType = "ability"
    DamageSourceRage        DamageSourceType = "rage"
    DamageSourceSneakAttack DamageSourceType = "sneak_attack"  // ADD NEW
)
```

### Step 2: Create the Condition
File: `rulebooks/dnd5e/conditions/<name>.go`

```go
type SneakAttackCondition struct {
    CharacterID     string
    DamageDice      string  // e.g., "3d6"
    subscriptionIDs []string
    bus             events.EventBus
}

// Implement ConditionBehavior interface
func (s *SneakAttackCondition) Apply(ctx context.Context, bus events.EventBus) error {
    s.bus = bus

    // Subscribe to damage chain
    damageChain := combat.DamageChain.On(bus)
    subID, err := damageChain.SubscribeWithChain(ctx, s.onDamageChain)
    if err != nil {
        return err
    }
    s.subscriptionIDs = append(s.subscriptionIDs, subID)

    return nil
}

func (s *SneakAttackCondition) onDamageChain(
    ctx context.Context,
    event *combat.DamageChainEvent,
    c chain.Chain[*combat.DamageChainEvent],
) (chain.Chain[*combat.DamageChainEvent], error) {
    // Only add if we're the attacker
    if event.AttackerID != s.CharacterID {
        return c, nil
    }

    // Add modifier at StageFeatures
    modifyDamage := func(_ context.Context, e *combat.DamageChainEvent) (*combat.DamageChainEvent, error) {
        // Roll sneak attack dice...
        e.Components = append(e.Components, combat.DamageComponent{
            Source:     combat.DamageSourceSneakAttack,
            FlatBonus:  0,
            FinalDiceRolls: diceRolls,
            DamageType: e.DamageType,
        })
        return e, nil
    }

    return c.Add(dnd5e.StageFeatures, "sneak_attack", modifyDamage)
}
```

### Step 3: Create the Feature (if activatable)
File: `rulebooks/dnd5e/features/<name>.go`

```go
type SneakAttack struct {
    level int  // Determines dice count
}

func (s *SneakAttack) Activate(ctx context.Context, owner core.Entity, input FeatureInput) error {
    condition := &conditions.SneakAttackCondition{
        CharacterID: owner.GetID(),
        DamageDice:  s.calculateDice(),
    }

    // Publish condition
    topic := dnd5eEvents.ConditionAppliedTopic.On(input.Bus)
    return topic.Publish(ctx, dnd5eEvents.ConditionAppliedEvent{
        Target:    owner,
        Type:      dnd5eEvents.ConditionSneakAttack,
        Condition: condition,
    })
}
```

## Workflow 2: Adding an Attack Roll Modifier

**Examples:** Bless (+1d4), Bane (-1d4), Prone (disadvantage on ranged)

### The Pattern

Subscribe to `AttackChain` instead of `DamageChain`:

```go
func (b *BlessedCondition) Apply(ctx context.Context, bus events.EventBus) error {
    attackChain := combat.AttackChain.On(bus)
    subID, err := attackChain.SubscribeWithChain(ctx, b.onAttackChain)
    // ...
}

func (b *BlessedCondition) onAttackChain(
    ctx context.Context,
    event AttackChainEvent,
    c chain.Chain[AttackChainEvent],
) (chain.Chain[AttackChainEvent], error) {
    if event.AttackerID != b.CharacterID {
        return c, nil
    }

    // Add at StageConditions (spell effects go here)
    modifyAttack := func(_ context.Context, e AttackChainEvent) (AttackChainEvent, error) {
        // Roll 1d4, add to attack bonus
        e.AttackBonus += blessBonus
        return e, nil
    }

    return c.Add(dnd5e.StageConditions, "bless", modifyAttack)
}
```

## Workflow 3: Adding a Passive Tracker

**Examples:** Rage's "did I attack this turn?", Concentration tracking

### The Pattern

Subscribe to TypedTopics (not chains) to track state:

```go
func (r *RagingCondition) Apply(ctx context.Context, bus events.EventBus) error {
    // Track attacks
    attacks := dnd5eEvents.AttackTopic.On(bus)
    subID1, err := attacks.Subscribe(ctx, r.onAttack)

    // Track being hit
    damages := dnd5eEvents.DamageReceivedTopic.On(bus)
    subID2, err := damages.Subscribe(ctx, r.onDamageReceived)

    // Check at turn end
    turnEnds := dnd5eEvents.TurnEndTopic.On(bus)
    subID3, err := turnEnds.Subscribe(ctx, r.onTurnEnd)
}

func (r *RagingCondition) onAttack(_ context.Context, event dnd5eEvents.AttackEvent) error {
    if event.AttackerID == r.CharacterID {
        r.DidAttackThisTurn = true
    }
    return nil
}

func (r *RagingCondition) onTurnEnd(ctx context.Context, event dnd5eEvents.TurnEndEvent) error {
    if !r.DidAttackThisTurn && !r.WasHitThisTurn {
        // End the condition
        removals := dnd5eEvents.ConditionRemovedTopic.On(r.bus)
        return removals.Publish(ctx, dnd5eEvents.ConditionRemovedEvent{...})
    }
    r.DidAttackThisTurn = false
    r.WasHitThisTurn = false
    return nil
}
```

## Workflow 4: Adding a New Event Topic

**Examples:** Rest events, spell casting events, movement events

### Step 1: Define the Event
File: `rulebooks/dnd5e/events/events.go`

```go
type RestCompleteEvent struct {
    CharacterID string
    RestType    string // "short" or "long"
}

var RestCompleteTopic = events.DefineTypedTopic[RestCompleteEvent]("dnd5e.rest.complete")
```

### Step 2: Publish where appropriate
```go
// In rest resolution code
topic := dnd5eEvents.RestCompleteTopic.On(bus)
topic.Publish(ctx, dnd5eEvents.RestCompleteEvent{
    CharacterID: char.GetID(),
    RestType:    "long",
})
```

### Step 3: Subscribe in conditions that care
```go
func (r *RageFeature) Apply(ctx context.Context, bus events.EventBus) error {
    rests := dnd5eEvents.RestCompleteTopic.On(bus)
    return rests.Subscribe(ctx, r.onRestComplete)
}

func (r *RageFeature) onRestComplete(ctx context.Context, e dnd5eEvents.RestCompleteEvent) error {
    if e.RestType == "long" {
        r.resource.RestoreToFull()
    }
    return nil
}
```

## Workflow 5: Adding a New Chain Topic

**Examples:** Saving throw modifiers, healing modifiers, AC calculation

### Step 1: Define the chain event and topic
File: `rulebooks/dnd5e/combat/<topic>.go`

```go
type SavingThrowChainEvent struct {
    CharacterID string
    Ability     abilities.Ability
    DC          int
    Bonus       int
    HasAdvantage bool
    HasDisadvantage bool
}

var SavingThrowChain = events.DefineChainedTopic[*SavingThrowChainEvent]("dnd5e.combat.save.chain")
```

### Step 2: Use in resolution code
```go
func ResolveSavingThrow(ctx context.Context, input *SaveInput) (*SaveResult, error) {
    event := &SavingThrowChainEvent{
        CharacterID: input.Character.GetID(),
        Ability:     input.Ability,
        DC:          input.DC,
        Bonus:       baseBonus,
    }

    chain := events.NewStagedChain[*SavingThrowChainEvent](dnd5e.ModifierStages)
    saves := SavingThrowChain.On(input.EventBus)

    modifiedChain, err := saves.PublishWithChain(ctx, event, chain)
    finalEvent, err := modifiedChain.Execute(ctx, event)

    // Use finalEvent.Bonus, finalEvent.HasAdvantage, etc.
}
```

## The Stage Selection Guide

| Stage | Use For | Examples |
|-------|---------|----------|
| `StageBase` | Initial values, proficiency | Base attack bonus, ability modifier |
| `StageFeatures` | Class/race features | Rage damage, Sneak Attack, Brutal Critical |
| `StageConditions` | Spell effects, status effects | Bless, Bane, Frightened, Poisoned |
| `StageEquipment` | Magic items, gear | +1 weapon, Ring of Protection |
| `StageFinal` | Resistance, vulnerability, caps | Damage resistance, minimum damage |

## Barbarian: Current State & What's Missing

### Implemented
- **Rage** (Level 1) - Full implementation with damage bonus, duration, auto-end

### Not Yet Implemented

| Feature | Level | Type | Infrastructure Needed |
|---------|-------|------|----------------------|
| **Unarmored Defense** | 1 | Passive | AC calculation chain |
| **Reckless Attack** | 2 | Action | Advantage/disadvantage system |
| **Danger Sense** | 2 | Passive | Saving throw chain |
| **Extra Attack** | 5 | Passive | Action economy system |
| **Fast Movement** | 5 | Passive | Movement calculation |
| **Feral Instinct** | 7 | Passive | Initiative chain |
| **Brutal Critical** | 9 | Passive | Critical damage chain (have DamageChain) |
| **Relentless Rage** | 11 | Reaction | Death save intervention |
| **Persistent Rage** | 15 | Passive | Modify RagingCondition |

### Recommended Implementation Order

1. **Brutal Critical** - Already have DamageChain, just need to check `IsCritical`
2. **Persistent Rage** - Modify existing RagingCondition
3. **Unarmored Defense** - Need AC calculation chain (useful for many classes)
4. **Reckless Attack** - Need advantage/disadvantage (useful everywhere)
5. **Danger Sense** - Need saving throw chain (useful everywhere)

## Implementation Checklist Template

When adding a new feature:

- [ ] Identify feature type (damage mod, attack mod, passive tracker, etc.)
- [ ] Choose appropriate workflow from above
- [ ] Add any needed constants (DamageSourceType, ConditionType, etc.)
- [ ] Create condition file if needed (`conditions/<name>.go`)
- [ ] Implement `ConditionBehavior` interface (Apply, Remove, ToJSON)
- [ ] Create feature file if activatable (`features/<name>.go`)
- [ ] Subscribe to correct topics at correct stages
- [ ] Add tests for condition behavior
- [ ] Add integration test showing full flow
- [ ] Update loader if using JSON persistence

## The Philosophy

**Features activate, Conditions modify.**

Keep this separation clean:
- Features manage resources (uses per rest, action economy)
- Features publish events when activated
- Conditions subscribe to events and modify game mechanics
- The event bus is the glue - everything flows through it

This architecture means we can add new features without modifying existing code. New conditions just subscribe to existing events. New events can be subscribed to by existing conditions. Loose coupling, high cohesion.

## Next Steps

1. Pick the next barbarian feature (recommend Brutal Critical)
2. Follow the appropriate workflow
3. Add tests
4. Document any new patterns discovered
5. Update this journey doc if workflows evolve

The goal: Make adding a new feature a ~30-minute task, not a multi-day adventure.
