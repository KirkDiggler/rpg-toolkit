# Abilities, Actions, and ActionEconomy Design

**Date**: 2026-01-07
**Status**: Proposed
**Related**: ADR-0028 (Typed Activatable Actions), ADR-0027 (Attack Resolution and Reactions)

## Summary

This design establishes the structure for activatable game mechanics in the D&D 5e rulebook. It defines how **Abilities** consume action economy to grant capacity, how **Actions** consume that capacity to do things, and how **Features** fit alongside as class/race-granted abilities.

## Core Concepts

### The Two-Level Economy

D&D 5e has two levels of resource consumption:

1. **Action Economy** - What you spend to do things (action, bonus action, reaction)
2. **Capacity** - What you get to actually do (attacks, movement)

Example: Taking the Attack action (spends action economy) grants you attacks (capacity). Each Strike (action) consumes one attack from that capacity.

### Type Definitions

| Type | Purpose | Implements | Stored On Character |
|------|---------|------------|---------------------|
| **Ability** | Universal activatable (Attack, Dash, Dodge) | `core.Action[AbilityInput]` | `abilities []Ability` |
| **Feature** | Class/race granted activatable (Rage, Flurry of Blows) | `core.Action[FeatureInput]` | `features []Feature` |
| **Action** | The doing (Strike, Move, FlurryStrike) | `core.Action[ActionInput]` | `actions []Action` |
| **Spell** | Magical effects (future) | `core.Action[SpellInput]` | `spells []Spell` |
| **Condition** | Passive effects (Raging, Dodging) | Event subscriber | `conditions []Condition` |

### Key Insight: Abilities/Features Grant, Actions Do

```
Ability/Feature activated
    → Consumes action economy (action, bonus action, reaction)
    → Grants capacity (AttacksRemaining, MovementRemaining)
    → And/or grants Actions (Strike, FlurryStrike)
    → And/or grants Conditions (Dodging, Raging)

Action activated
    → Consumes capacity (AttacksRemaining, MovementRemaining)
    → Does the thing (damage, movement, effect)
```

## ActionEconomy Structure

```go
type ActionEconomy struct {
    // Primary resources (consumed by abilities/features)
    ActionsRemaining      int  // Default 1, reset on turn start
    BonusActionsRemaining int  // Default 1, reset on turn start
    ReactionsRemaining    int  // Default 1, reset on turn start

    // Capacity (set when specific abilities are used)
    AttacksRemaining      int  // Set when Attack ability is taken
    MovementRemaining     int  // Set at turn start from character speed
}
```

### Turn Start Flow

1. `Reset()` sets actions/bonus/reactions to 1
2. `MovementRemaining` set from character's speed
3. `AttacksRemaining` stays 0 until Attack ability is taken

### Attack Ability Flow

```
Attack ability activated
    → ActionsRemaining: 1 → 0
    → AttacksRemaining: 0 → N (1 normally, 2+ with Extra Attack)
    → Strike action(s) granted to character

Strike action activated (per attack)
    → AttacksRemaining: N → N-1
    → Target selection, attack resolution (ADR-0027)
    → TwoWeaponGranter checks for off-hand (if dual wielding)
```

### Dash Ability Flow

```
Dash ability activated
    → ActionsRemaining: 1 → 0
    → MovementRemaining: += character.Speed

Move action (anytime during turn)
    → MovementRemaining: N → N-distance
    → Position changes
```

## Standard Abilities

Every character gets these when transitioning from draft to character:

| Ability | Costs | Effect |
|---------|-------|--------|
| Attack | 1 action | Sets AttacksRemaining, grants Strike action(s) |
| Dash | 1 action | Adds speed to MovementRemaining |
| Dodge | 1 action | Grants Dodging condition |
| Disengage | 1 action | Grants Disengaging condition |
| Help | 1 action | Grants advantage to ally |
| Hide | 1 action | Attempt stealth check |
| Ready | 1 action | Prepare action for trigger |

## Actions

Actions are the "doing" - they consume capacity and resolve effects.

### Strike Action

```go
type Strike struct {
    id        string
    ownerID   string
    weaponID  string
}

func (s *Strike) CanActivate(ctx context.Context, owner core.Entity, input ActionInput) error {
    if input.ActionEconomy.AttacksRemaining <= 0 {
        return errors.New("no attacks remaining")
    }
    if input.Target == nil {
        return errors.New("target required")
    }
    return nil
}

func (s *Strike) Activate(ctx context.Context, owner core.Entity, input ActionInput) error {
    input.ActionEconomy.AttacksRemaining--

    // Actually resolve the attack (ADR-0027)
    // Events (AttackDeclared, AttackResolved, DamageApplied) are published
    // during resolution for observers/reactions
    _, err := combat.ResolveAttack(ctx, input.Bus, &ResolveAttackInput{
        AttackerID: s.ownerID,
        TargetID:   input.Target.GetID(),
        WeaponID:   s.weaponID,
    })

    return err
}
```

### Move Action

```go
type Move struct {
    id      string
    ownerID string
}

func (m *Move) CanActivate(ctx context.Context, owner core.Entity, input ActionInput) error {
    if input.ActionEconomy.MovementRemaining <= 0 {
        return errors.New("no movement remaining")
    }
    if input.Destination == nil {
        return errors.New("destination required")
    }
    // Check path cost
    cost := calculateMovementCost(owner.GetPosition(), input.Destination)
    if cost > input.ActionEconomy.MovementRemaining {
        return fmt.Errorf("insufficient movement: need %d, have %d", cost, input.ActionEconomy.MovementRemaining)
    }
    return nil
}

func (m *Move) Activate(ctx context.Context, owner core.Entity, input ActionInput) error {
    cost := calculateMovementCost(owner.GetPosition(), input.Destination)
    input.ActionEconomy.MovementRemaining -= cost

    // Actually execute the movement
    // This publishes step-by-step MovementStep events for AoO detection (ADR-0027)
    // and updates the entity's position in gamectx
    err := spatial.ExecuteMovement(ctx, input.Bus, &ExecuteMovementInput{
        EntityID:    m.ownerID,
        Destination: input.Destination,
    })

    return err
}
```

## Feature Integration

Features (class/race granted) follow the same pattern as Abilities:

### Flurry of Blows (Monk)

```go
func (f *FlurryOfBlows) Activate(ctx context.Context, owner core.Entity, input FeatureInput) error {
    // Consume Ki
    if err := owner.UseResource(resources.Ki, 1); err != nil {
        return err
    }

    // Grant two FlurryStrike actions
    strike1 := NewFlurryStrike(owner.GetID())
    strike2 := NewFlurryStrike(owner.GetID())

    strike1.Apply(ctx, input.Bus)
    strike2.Apply(ctx, input.Bus)

    // Publish for character to add
    input.Bus.Publish(ctx, &ActionGrantedEvent{CharacterID: owner.GetID(), Action: strike1})
    input.Bus.Publish(ctx, &ActionGrantedEvent{CharacterID: owner.GetID(), Action: strike2})

    return nil
}
```

### Rage (Barbarian)

```go
func (r *Rage) Activate(ctx context.Context, owner core.Entity, input FeatureInput) error {
    // Consume rage charge
    if err := owner.UseResource(resources.RageCharges, 1); err != nil {
        return err
    }

    // Grant Raging condition
    condition := conditions.NewRaging(owner.GetID(), r.damageBonus)
    condition.Apply(ctx, input.Bus)

    input.Bus.Publish(ctx, &ConditionAppliedEvent{CharacterID: owner.GetID(), Condition: condition})

    return nil
}
```

## Two-Weapon Fighting

Off-hand attacks are granted by a condition that listens for attacks:

### DualWieldingCondition

Applied when character equips two light weapons:

```go
func (d *DualWieldingCondition) Apply(ctx context.Context, bus events.EventBus) error {
    // Subscribe to attack resolution
    AttackResolvedTopic.On(bus).Subscribe(ctx, d.onAttackResolved)
    return nil
}

func (d *DualWieldingCondition) onAttackResolved(ctx context.Context, event AttackResolvedEvent) error {
    if event.AttackerID != d.characterID {
        return nil
    }

    // Check if main hand attack with light weapon
    if !d.isMainHandLightWeapon(event.WeaponID) {
        return nil
    }

    // Check if off-hand is also light
    if !d.hasLightOffHand() {
        return nil
    }

    // Grant OffHandStrike action
    action := NewOffHandStrike(d.characterID, d.offHandWeaponID)
    action.Apply(ctx, d.bus)

    d.bus.Publish(ctx, &ActionGrantedEvent{
        CharacterID: d.characterID,
        Action:      action,
    })

    return nil
}
```

### OffHandStrike Action

```go
type OffHandStrike struct {
    id             string
    ownerID        string
    weaponID       string
    subscriptionID string
}

func (o *OffHandStrike) CanActivate(ctx context.Context, owner core.Entity, input ActionInput) error {
    if err := input.ActionEconomy.CanUseBonusAction(); err != nil {
        return err
    }
    return nil
}

func (o *OffHandStrike) Activate(ctx context.Context, owner core.Entity, input ActionInput) error {
    if err := input.ActionEconomy.UseBonusAction(); err != nil {
        return err
    }

    // Actually resolve the attack
    _, err := combat.ResolveAttack(ctx, input.Bus, &ResolveAttackInput{
        AttackerID: o.ownerID,
        TargetID:   input.Target.GetID(),
        WeaponID:   o.weaponID,
        IsOffHand:  true,  // Damage chain checks this for ability modifier
    })
    if err != nil {
        return err
    }

    // Remove self after use
    o.Remove(ctx, input.Bus)

    return nil
}

func (o *OffHandStrike) Apply(ctx context.Context, bus events.EventBus) error {
    // Subscribe to turn end for cleanup
    subID, _ := TurnEndTopic.On(bus).Subscribe(ctx, o.onTurnEnd)
    o.subscriptionID = subID
    return nil
}

func (o *OffHandStrike) onTurnEnd(ctx context.Context, event TurnEndEvent) error {
    if event.CharacterID == o.ownerID {
        return o.Remove(ctx, o.bus)
    }
    return nil
}
```

## Event Flow for Action Granting

Actions are granted via events, allowing observability and UI updates:

```go
// Character subscribes to action events
func (c *Character) SubscribeEvents(ctx context.Context, bus events.EventBus) error {
    ActionGrantedTopic.On(bus).Subscribe(ctx, c.onActionGranted)
    ActionRemovedTopic.On(bus).Subscribe(ctx, c.onActionRemoved)
    ConditionAppliedTopic.On(bus).Subscribe(ctx, c.onConditionApplied)
    // ... other subscriptions
    return nil
}

func (c *Character) onActionGranted(ctx context.Context, event ActionGrantedEvent) error {
    if event.CharacterID != c.id {
        return nil
    }
    return c.AddAction(event.Action)
}

func (c *Character) onActionRemoved(ctx context.Context, event ActionRemovedEvent) error {
    if event.CharacterID != c.id {
        return nil
    }
    return c.RemoveAction(event.ActionID)
}
```

## Example: Full Combat Turn

```
Turn starts for Fighter (Extra Attack, dual wielding)
    → ActionsRemaining: 1
    → BonusActionsRemaining: 1
    → ReactionsRemaining: 1
    → AttacksRemaining: 0
    → MovementRemaining: 30

1. Fighter activates Move toward goblin (15ft)
    → MovementRemaining: 30 → 15
    → Position changes

2. Fighter activates Attack ability
    → ActionsRemaining: 1 → 0
    → AttacksRemaining: 0 → 2 (Extra Attack)
    → Strike actions granted

3. Fighter activates Strike on goblin
    → AttacksRemaining: 2 → 1
    → Attack resolves (ADR-0027), hits for 8 damage
    → DualWieldingCondition sees attack, grants OffHandStrike

4. Fighter activates Move to flank (5ft)
    → MovementRemaining: 15 → 10

5. Fighter activates Strike on goblin
    → AttacksRemaining: 1 → 0
    → Attack resolves, hits for 10 damage

6. Fighter activates OffHandStrike on goblin
    → BonusActionsRemaining: 1 → 0
    → Attack resolves, hits for 4 damage
    → OffHandStrike removes itself

Turn ends
    → Any remaining temporary actions clean up
```

## UI Representation

The UI displays different sections based on source:

| Section | Source | Contents |
|---------|--------|----------|
| Combat Actions | `abilities` | Attack, Dash, Dodge (universal) |
| Features | `features` | Rage, Flurry of Blows (class/race) |
| Available Now | `actions` | Strike x2, OffHandStrike (granted, contextual) |
| Active Conditions | `conditions` | Raging, Dodging (passive effects) |
| Spells | `spells` | Fireball, Cure Wounds (future) |

## Spells (Future)

Spells will implement `core.Action[SpellInput]` with unique input:

```go
type SpellInput struct {
    Bus           events.EventBus
    ActionEconomy *ActionEconomy
    SlotLevel     int              // Spell slot consumed
    Targets       []core.Entity    // Multi-target support
    Origin        *spatial.Coord   // For AoE placement
}
```

Most spells are direct (do effect immediately), unlike Attack which grants capacity. This fits the model - spells are Abilities that do things directly rather than granting Actions.

## Implementation Order

1. Update `ActionEconomy` with `AttacksRemaining` and `MovementRemaining`
2. Create `Ability` interface and base implementation
3. Implement standard abilities (Attack, Dash, Dodge, Disengage)
4. Create `Strike` and `Move` actions
5. Add `abilities []Ability` to Character
6. Implement draft → character transition that adds standard abilities
7. Add `ActionGrantedEvent` and `ActionRemovedEvent`
8. Update `Character.SubscribeEvents()` to handle action events
9. Refactor `DualWieldingCondition` to use event-based action granting
10. Update existing features (FlurryOfBlows) to use new pattern

## References

- ADR-0027: Attack Resolution and Reactions
- ADR-0028: Typed Activatable Actions
- Issue #505: Implement attack resolution and reactions
- Issue #399: Add ActionEconomy history tracking
- Issue #360: Fighting Styles
- Issue #529: Implement Abilities and Actions System
