# Attack Chain Advantage/Disadvantage Design

**Issue:** #436
**Date:** 2025-12-15
**Status:** Approved

## Problem

The current attack flow rolls the d20 BEFORE firing the AttackChain, making it impossible to impose advantage or disadvantage from chain listeners. This blocks reaction-based mechanics like Protection fighting style.

Current flow:
```
1. AttackEvent (notification)
2. Roll d20              <- Too early!
3. AttackChain           <- Can't affect the roll
4. Determine hit
5. DamageChain
```

## Solution

Move AttackChain to fire BEFORE the d20 roll. Add advantage/disadvantage source tracking and reaction consumption output.

New flow:
```
1. AttackChain           <- Collects adv/disadv sources, consumes reactions
2. Roll d20              <- Uses chain result to determine roll mode
3. Determine hit
4. DamageChain           <- If hit
5. Publish ReactionUsedEvent for each consumption
```

## Data Structures

### AttackModifierSource
Tracks who provided advantage or disadvantage and why.

```go
type AttackModifierSource struct {
    SourceRef *core.Ref  // refs.FightingStyles.Protection()
    SourceID  string     // Character who provided it
    Reason    string     // "Protection fighting style reaction"
}
```

### ReactionConsumption
Tracks reactions consumed during the chain for side-effect processing after.

```go
type ReactionConsumption struct {
    CharacterID string     // Who used their reaction
    FeatureRef  *core.Ref  // What feature consumed it
    Reason      string     // Human-readable
}
```

### Enhanced AttackChainEvent

```go
type AttackChainEvent struct {
    // Identity
    AttackerID string
    TargetID   string
    WeaponRef  *core.Ref
    IsMelee    bool

    // Advantage/Disadvantage (input to roll)
    AdvantageSources    []AttackModifierSource
    DisadvantageSources []AttackModifierSource

    // Modifiers (applied after roll)
    AttackBonus       int
    CriticalThreshold int  // Default 20, Champion can lower

    // Side effects (processed after chain)
    ReactionsConsumed []ReactionConsumption
}
```

**Removed fields:** `AttackRoll`, `IsNaturalTwenty`, `IsNaturalOne` - these are outputs of the roll, not inputs to the chain.

## Roll Logic

D&D 5e rule: Any advantage + any disadvantage = normal roll (they cancel completely).

```go
hasAdv := len(event.AdvantageSources) > 0
hasDisadv := len(event.DisadvantageSources) > 0

var attackRoll int
var rolls []int

switch {
case hasAdv && hasDisadv:
    attackRoll, _ = roller.Roll(ctx, 20)
case hasAdv:
    attackRoll, rolls = roller.RollWithAdvantage(ctx, 20)
case hasDisadv:
    attackRoll, rolls = roller.RollWithDisadvantage(ctx, 20)
default:
    attackRoll, _ = roller.Roll(ctx, 20)
}
```

## Dice Roller Extensions

Add to `dice.Roller` interface:

```go
type Roller interface {
    Roll(ctx context.Context, sides int) (int, error)
    RollN(ctx context.Context, n, sides int) ([]int, error)
    // New methods
    RollWithAdvantage(ctx context.Context, sides int) (result int, rolls []int, err error)
    RollWithDisadvantage(ctx context.Context, sides int) (result int, rolls []int, err error)
}
```

Returns both the chosen result and all rolls (for UI display: "Rolled 15, 8 - took 15").

## Reaction Flow

1. **Check availability:** Protection reads ActionEconomy from gamectx (read-only)
2. **Consume in chain:** Adds to `ReactionsConsumed` and `DisadvantageSources`
3. **Process after chain:** `attack.go` publishes `ReactionUsedEvent` for each
4. **Update state:** Game server listens, updates ActionEconomy

This keeps chain execution pure (no side effects during) while enabling state changes after.

## Protection Fighting Style Example

```go
func (p *ProtectionCondition) onAttackChain(
    ctx context.Context,
    event *dnd5eEvents.AttackChainEvent,
    c chain.Chain[*dnd5eEvents.AttackChainEvent],
) (chain.Chain[*dnd5eEvents.AttackChainEvent], error) {
    // Only react to attacks on others
    if event.TargetID == p.CharacterID {
        return c, nil
    }

    // Must be melee attack
    if !event.IsMelee {
        return c, nil
    }

    // Check if target is within 5ft (ally we're protecting)
    if !p.isTargetWithin5ft(ctx, event.TargetID) {
        return c, nil
    }

    // Check if we have shield equipped
    if !p.hasShieldEquipped(ctx) {
        return c, nil
    }

    // Check if reaction available
    if !p.hasReactionAvailable(ctx) {
        return c, nil
    }

    // Add disadvantage and consume reaction
    modifier := func(_ context.Context, e *dnd5eEvents.AttackChainEvent) (*dnd5eEvents.AttackChainEvent, error) {
        e.DisadvantageSources = append(e.DisadvantageSources, dnd5eEvents.AttackModifierSource{
            SourceRef: refs.FightingStyles.Protection(),
            SourceID:  p.CharacterID,
            Reason:    "Protection fighting style reaction",
        })
        e.ReactionsConsumed = append(e.ReactionsConsumed, dnd5eEvents.ReactionConsumption{
            CharacterID: p.CharacterID,
            FeatureRef:  refs.FightingStyles.Protection(),
            Reason:      "Imposed disadvantage on attack",
        })
        return e, nil
    }

    return c.Add(combat.StageFeatures, "protection", modifier), nil
}
```

## Implementation Steps

1. Add `AttackModifierSource` and `ReactionConsumption` to `events/events.go`
2. Enhance `AttackChainEvent` with new fields, remove roll-output fields
3. Add `RollWithAdvantage`/`RollWithDisadvantage` to `dice.Roller`
4. Update `attack.go` to fire chain before roll, process ReactionsConsumed after
5. Add `ReactionUsedEvent` topic
6. Deprecate `AttackEvent` (chain serves both purposes now)
7. Update existing AttackChain subscribers (Archery fighting style, etc.)
8. Write tests

## Deprecations

- `AttackEvent` - Replaced by chainable `AttackChainEvent`
- `AttackChainEvent.AttackRoll` - Now an output, not part of event
- `AttackChainEvent.IsNaturalTwenty` - Now an output
- `AttackChainEvent.IsNaturalOne` - Now an output

## Test Cases

1. Chain fires before roll
2. Advantage source makes roll take higher of two
3. Disadvantage source makes roll take lower of two
4. Advantage + disadvantage cancels to normal roll
5. Multiple advantage sources still just roll with advantage (no stacking)
6. ReactionsConsumed populated when reaction used
7. ReactionUsedEvent published after chain execution
