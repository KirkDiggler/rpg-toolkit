# ADR-0028: Typed Activatable Actions

## Status
Proposed

## Context

D&D 5e characters have things they can **do** (activatable) and things that **affect them** (passive):

**Activatable:**
- **Features**: Rage, Second Wind, Flurry of Blows - class/race abilities
- **Actions**: Attack, Dash, Flurry Strike - things you do on your turn
- **Spells**: Fireball, Cure Wounds - magical effects (future)

**Passive:**
- **Conditions**: Raging, Poisoned, Stunned - effects applied to you

The key insight is that **Features grant Actions and Conditions**. Flurry of Blows (feature) grants two Flurry Strike actions. Rage (feature) applies the Raging condition.

Currently, features implement `core.Action[FeatureInput]`:

```go
type FeatureInput struct {
    Bus           events.EventBus
    ActionEconomy *combat.ActionEconomy
    Action        string  // For features with choices
}
```

But actions like Flurry Strike need different input - specifically a target:

```go
type ActionInput struct {
    Bus           events.EventBus
    ActionEconomy *combat.ActionEconomy
    Target        core.Entity
    Position      *spatial.Position  // For AoE/point targeting
}
```

## Decision

### 1. Three Domain Types

| Type | Purpose | Implements | Stored On Character |
|------|---------|------------|---------------------|
| Feature | Grants actions/conditions | `core.Action[FeatureInput]` | Yes - `features []Feature` |
| Action | Something you do | `core.Action[ActionInput]` | Yes - `actions []Action` |
| Condition | Passive effect on you | Event subscriber (not Action) | Yes - `conditions []Condition` |

**Conditions are NOT activatable.** They don't implement `core.Action[T]`. They're applied to you and modify your state/rolls passively.

### 2. Actions Are First-Class and Grantable

Actions live on the character and can be:
- **Permanent**: Attack, Dash, Dodge (always available)
- **Temporary**: Flurry Strike, Haste Attack (granted, expire)

```go
type Character struct {
    features   []features.Feature      // Rage, Flurry of Blows
    actions    []actions.Action        // Attack, Dash, Flurry Strike (if granted)
    conditions []conditions.Condition  // Raging, Poisoned
}

type Action interface {
    core.Action[ActionInput]

    // Event lifecycle
    Apply(ctx context.Context, bus events.EventBus) error
    Remove(ctx context.Context, bus events.EventBus) error

    // Lifecycle metadata
    IsTemporary() bool
    UsesRemaining() int  // -1 = unlimited
}
```

### 3. Features Grant Actions

When you activate Flurry of Blows, it grants temporary actions:

```go
func (f *FlurryOfBlows) Activate(ctx context.Context, owner core.Entity, input FeatureInput) error {
    // 1. Check and spend Ki
    if err := owner.UseResource(resources.Ki, 1); err != nil {
        return err
    }

    // 2. Grant two Flurry Strike actions
    strike1 := NewFlurryStrike(owner.GetID(), 1)
    strike2 := NewFlurryStrike(owner.GetID(), 1)

    // Actions subscribe to events for self-removal
    if err := strike1.Apply(ctx, input.Bus); err != nil {
        return err
    }
    if err := strike2.Apply(ctx, input.Bus); err != nil {
        return err
    }

    // 3. Add to character's available actions
    owner.AddAction(strike1)
    owner.AddAction(strike2)

    return nil
}
```

### 4. Actions and Conditions Share Event Lifecycle Pattern

Both use Apply/Remove for event subscriptions:

```go
// FlurryStrike subscribes to turn end for auto-cleanup
func (f *FlurryStrike) Apply(ctx context.Context, bus events.EventBus) error {
    f.bus = bus

    turnEndTopic := combat.TurnEndTopic.On(bus)
    subID, err := turnEndTopic.Subscribe(ctx, f.onTurnEnd)
    if err != nil {
        return err
    }
    f.subscriptionID = subID
    return nil
}

func (f *FlurryStrike) onTurnEnd(ctx context.Context, event combat.TurnEndEvent) error {
    if event.CharacterID == f.ownerID {
        // Remove self from character and unsubscribe
        return f.Remove(ctx, f.bus)
    }
    return nil
}
```

### 5. Unified Pattern

| Type | Activatable | Apply/Remove | Self-Removes On |
|------|-------------|--------------|-----------------|
| Feature | Yes (grants things) | Sometimes | Rarely |
| Action | Yes (does something) | Yes | Turn end, after use |
| Condition | No (passive) | Yes | Rest, duration, dispel |

**Features grant. Actions do. Conditions modify.**

## Consequences

### Positive
- Clear separation: Features grant, Actions do, Conditions modify
- Actions are first-class citizens with their own slice
- Temporary actions handle their own lifecycle via events
- UI simply renders `character.GetActions()` - no special cases
- Same Apply/Remove pattern as conditions - consistent architecture

### Negative
- Three types to understand (but they're clearly different)
- Actions need to manage their own subscriptions

### Neutral
- Character has multiple typed slices
- Features don't "do" things directly - they grant the ability to do things

## Example: Flurry of Blows Flow

```
1. Player takes Attack action (unarmed or monk weapon)
   → Normal attack resolution

2. Player clicks "Flurry of Blows" (feature)
   → Feature.Activate() called
   → Spends 1 Ki
   → Grants 2 FlurryStrike actions to character
   → Each FlurryStrike subscribes to turn end

3. UI shows character.GetActions()
   → Player sees "Flurry Strike" x2 available

4. Player clicks "Flurry Strike 1", selects target
   → Action.Activate(target) called
   → Roll to hit, damage on hit
   → UsesRemaining decrements to 0
   → Action removes itself

5. Player clicks "Flurry Strike 2", selects different target
   → Same resolution

6. Turn ends
   → Any unused FlurryStrike actions receive TurnEnd event
   → They remove themselves
```

## Example: Two-Weapon Fighting Flow

TWF off-hand attack could be modeled as a granted action:

```
1. Player makes Attack action with main hand weapon
   → ResolveAttack() checks: dual wielding light weapons?
   → If yes, grants OffHandStrike action (temporary, 1 use)

2. UI shows "Off-Hand Attack" in available actions

3. Player clicks "Off-Hand Attack"
   → Action.Activate(target) called
   → Uses bonus action
   → Validates light weapons
   → Damage without STR mod (unless TWF fighting style)
   → Action removes itself
```

## Example: Rage Flow

Rage grants a condition, not actions:

```
1. Player clicks "Rage" (feature)
   → Feature.Activate() called
   → Spends 1 rage use
   → Applies RagingCondition to character

2. RagingCondition.Apply() subscribes to:
   → DamageDealt (add rage damage)
   → DamageTaken (apply resistance)
   → TurnEnd (check if raged this turn)
   → Rest (remove on rest)

3. Condition passively modifies damage/resistance
   → No player interaction needed
```

## Implementation Order

1. Define `Action` interface with `ActionInput`
2. Add `actions []Action` to Character
3. Implement `AddAction()`, `RemoveAction()`, `GetActions()`
4. Create `FlurryStrike` as proof of concept
5. Update `FlurryOfBlows` feature to grant actions
6. Add turn end cleanup via events

## References
- ADR-0020: Features and Conditions Simplification
- ADR-0021: Actions as Internal Implementation Pattern
- Issue #359: Support Two-Weapon Fighting
