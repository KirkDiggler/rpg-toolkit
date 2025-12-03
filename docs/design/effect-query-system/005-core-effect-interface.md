# Core Effect Interface Design

**Date:** 2024-12-01
**Status:** Draft
**Builds on:** 004-hybrid-design.md

## The Unifying Abstraction

We want `core.Effect` to be the foundation that all modifiers build on. Currently we have:

```go
// core/effect/types.go (current)
type Effect[T any] interface {
    Apply(chain chain.Chain[T]) error
    Remove(chain chain.Chain[T]) error
}
```

This is too narrow - it only works with chains. Let's rethink.

## What All Effects Share

Every effect (condition, passive, buff, equipment bonus) needs:

1. **Lifecycle**: Apply and Remove
2. **Identity**: Know what it is (for persistence, display)
3. **Active state**: Is it currently applied?

That's it. The **target** of the effect varies:
- Bus effects subscribe to topics
- Chain effects modify data flow
- Passive effects just exist (but still need apply/remove lifecycle)

## Proposed: Simple Core Effect

```go
// core/effect/effect.go

// Effect is the fundamental contract for anything that can be
// applied to and removed from the game state.
type Effect interface {
    // Apply activates this effect. What "activation" means
    // depends on the effect type - subscribing to events,
    // modifying a value, granting a capability, etc.
    Apply(ctx context.Context) error

    // Remove deactivates this effect, cleaning up any
    // subscriptions, modifications, or state.
    Remove(ctx context.Context) error

    // IsActive returns true if this effect is currently applied.
    IsActive() bool
}
```

Note: **No generic**. The target is an implementation detail.

## Persistence as Separate Concern

```go
// core/persist/persist.go

// Persistable can be serialized for storage.
// Effects that need to survive restarts implement this.
type Persistable interface {
    // ToJSON serializes the effect state
    ToJSON() (json.RawMessage, error)
}

// The Ref is embedded in the JSON for routing during load
// (This is already our pattern with core.Ref)
```

## Bus-Aware Effects

For effects that need the event bus (most conditions):

```go
// core/effect/bus.go

// BusEffect is an Effect that operates on an EventBus.
// Most game conditions are BusEffects - they subscribe to
// topics during Apply and unsubscribe during Remove.
type BusEffect interface {
    Effect

    // SetBus provides the event bus before Apply is called.
    // This allows the effect to store the bus reference for
    // later use (subscriptions, publishing).
    SetBus(bus events.EventBus)
}

// Helper for implementing BusEffect
type BaseBusEffect struct {
    bus             events.EventBus
    subscriptionIDs []string
    active          bool
}

func (b *BaseBusEffect) SetBus(bus events.EventBus) {
    b.bus = bus
}

func (b *BaseBusEffect) IsActive() bool {
    return b.active
}

func (b *BaseBusEffect) Bus() events.EventBus {
    return b.bus
}

func (b *BaseBusEffect) AddSubscription(id string) {
    b.subscriptionIDs = append(b.subscriptionIDs, id)
}

func (b *BaseBusEffect) RemoveAllSubscriptions(ctx context.Context) error {
    for _, id := range b.subscriptionIDs {
        if err := b.bus.Unsubscribe(ctx, id); err != nil {
            return err
        }
    }
    b.subscriptionIDs = nil
    return nil
}
```

## How Conditions Use It

```go
// rulebooks/dnd5e/conditions/raging.go

type RagingCondition struct {
    effect.BaseBusEffect  // Embed the helper

    CharacterID string
    DamageBonus int
    Level       int
    // ... other fields
}

// Ensure we implement the interfaces
var _ effect.BusEffect = (*RagingCondition)(nil)
var _ persist.Persistable = (*RagingCondition)(nil)

func (r *RagingCondition) Apply(ctx context.Context) error {
    if r.IsActive() {
        return rpgerr.New(rpgerr.CodeAlreadyExists, "already applied")
    }

    bus := r.Bus()

    // Subscribe to damage chain
    damageChain := combat.DamageChain.On(bus)
    subID, err := damageChain.SubscribeWithChain(ctx, r.onDamageChain)
    if err != nil {
        return err
    }
    r.AddSubscription(subID)

    // Subscribe to turn end
    turnEnds := dnd5eEvents.TurnEndTopic.On(bus)
    subID, err = turnEnds.Subscribe(ctx, r.onTurnEnd)
    if err != nil {
        _ = r.RemoveAllSubscriptions(ctx)
        return err
    }
    r.AddSubscription(subID)

    // ... more subscriptions

    r.active = true
    return nil
}

func (r *RagingCondition) Remove(ctx context.Context) error {
    if !r.IsActive() {
        return nil
    }

    if err := r.RemoveAllSubscriptions(ctx); err != nil {
        return err
    }

    r.active = false
    return nil
}

func (r *RagingCondition) ToJSON() (json.RawMessage, error) {
    data := RagingData{
        Ref: core.Ref{Module: "dnd5e", Type: "conditions", ID: "raging"},
        // ... fields
    }
    return json.Marshal(data)
}
```

## The ConditionBehavior Evolution

Currently `dnd5eEvents.ConditionBehavior` is:

```go
type ConditionBehavior interface {
    IsApplied() bool
    Apply(ctx context.Context, bus events.EventBus) error
    Remove(ctx context.Context, bus events.EventBus) error
    ToJSON() (json.RawMessage, error)
}
```

With the new pattern, it becomes:

```go
// rulebooks/dnd5e/conditions/types.go

// Condition is a BusEffect that can be persisted.
// This is the contract for all D&D 5e conditions.
type Condition interface {
    effect.BusEffect
    persist.Persistable
}
```

That's it. Much cleaner.

## Chain Effects (For Reference)

Some effects only modify chains, not subscribe to bus:

```go
// core/effect/chain.go

// ChainEffect modifies a specific type of chain.
// Used for pure modifiers that don't need bus subscriptions.
type ChainEffect[T any] interface {
    // AddToChain adds this effect's modifier to the chain
    AddToChain(c chain.Chain[T]) error

    // RemoveFromChain removes this effect's modifier
    RemoveFromChain(c chain.Chain[T]) error
}
```

Most game effects are BusEffects that happen to subscribe to chains. Pure ChainEffects are rare but possible.

## Applying Effects (Condition Manager Pattern)

```go
// rulebooks/dnd5e/conditions/manager.go

type ConditionManager struct {
    bus        events.EventBus
    conditions map[string]Condition  // Active conditions by ID
}

func (m *ConditionManager) ApplyCondition(ctx context.Context, c Condition) error {
    // Provide the bus
    c.SetBus(m.bus)

    // Apply the condition
    if err := c.Apply(ctx); err != nil {
        return err
    }

    // Track it
    m.conditions[c.GetID()] = c

    return nil
}

func (m *ConditionManager) RemoveCondition(ctx context.Context, id string) error {
    c, exists := m.conditions[id]
    if !exists {
        return nil
    }

    if err := c.Remove(ctx); err != nil {
        return err
    }

    delete(m.conditions, id)
    return nil
}
```

## Summary

| Interface | Package | Purpose |
|-----------|---------|---------|
| `Effect` | `core/effect` | Base lifecycle (Apply/Remove/IsActive) |
| `BusEffect` | `core/effect` | Effect + SetBus for event-based effects |
| `Persistable` | `core/persist` | ToJSON for storage |
| `Condition` | `dnd5e/conditions` | BusEffect + Persistable (D&D conditions) |

## Benefits

1. **Clear layering**: Core provides infrastructure, rulebook provides implementation
2. **Composable**: Effects can implement multiple interfaces
3. **Simple core**: Effect is just Apply/Remove/IsActive
4. **Flexible**: BusEffect, ChainEffect, or custom
5. **Consistent**: All conditions follow same pattern

## Migration Path

1. Add `core/effect.Effect` interface (simple version)
2. Add `core/effect.BusEffect` and `BaseBusEffect`
3. Add `core/persist.Persistable`
4. Update `dnd5e/conditions.Condition` to compose these
5. Update existing conditions to embed `BaseBusEffect`
6. Remove `dnd5eEvents.ConditionBehavior` (replaced by Condition)

## Open Questions

1. Should `Effect` have `GetID()` or is that separate (Entity)?
2. Do we need `ChainEffect[T]` or is it too specialized?
3. Should `BaseBusEffect` be in core or in rulebook?
