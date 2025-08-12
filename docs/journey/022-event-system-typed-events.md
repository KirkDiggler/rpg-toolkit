# Journey 022: Event System and Typed Events

## Date: 2025-08-12

## The Problem

After completing the features refactor, we discovered our event system has fundamental issues:

1. **Runtime type assertions everywhere**:
   ```go
   finalDamage := dmgEvent.Context().Get("damage").(int)  // Could panic!
   ```

2. **No first-class fields for domain concepts**:
   - A damage event without a `Damage` field
   - A spell cast event without a `SpellLevel` field
   - Everything is stringly-typed in a context map

3. **Smart subscriptions need enhancement**:
   - We added `ToTarget()` filtering
   - But still checking event relevance manually in handlers

4. **State tracking through boolean flags**:
   ```go
   type Character struct {
       hasDealtDamage bool  // This turn
       hasTakenDamage bool  // This turn
   }
   ```
   This works but doesn't scale. Event history might be better.

## The Core Question

How do we balance:
- Type safety (strongly typed events)
- Flexibility (new event types without changing core)
- Performance (filtering at subscription time)
- Simplicity (not over-engineering)

## Current State

```go
// What we have - generic events with context
type GameEvent interface {
    Type() string
    Source() core.Entity
    Target() core.Entity
    Context() Context  // map[string]interface{} underneath
}

// Usage
event := events.NewGameEvent("damage.before", source, target)
event.Context().Set("damage", 20)
event.Context().Set("damage_type", "slashing")
```

## What We Want

```go
// Strongly typed domain events
type DamageEvent struct {
    EventBase  // Common fields
    Damage     int
    DamageType DamageType
    IsCritical bool
    IsMagical  bool
}

// But still work with generic bus
bus.Publish(ctx, &DamageEvent{
    Damage:     20,
    DamageType: Slashing,
})

// And smart subscriptions
bus.On(TypeOf[DamageEvent]()).
    ToTarget(myID).
    Where(func(e *DamageEvent) bool {
        return e.DamageType == Slashing
    }).
    Do(func(e *DamageEvent) {
        // e is already typed!
        e.Damage = e.Damage / 2  // Resistance
    })
```

## Design Tensions

### 1. Type Safety vs Flexibility

**Option A: Interface + Type Assertion**
```go
type Event interface {
    Type() string
    Source() core.Entity
    Target() core.Entity
}

// Subscribe
bus.On("damage").Do(func(e Event) {
    dmg := e.(*DamageEvent)  // Assert
})
```

**Option B: Generics**
```go
func On[T Event](bus EventBus) Subscription[T] {
    return &TypedSubscription[T]{...}
}

bus.On[DamageEvent]().Do(func(e *DamageEvent) {
    // Already typed
})
```

**Option C: Event Visitors**
```go
type EventHandler interface {
    HandleDamage(*DamageEvent) error
    HandleSpellCast(*SpellCastEvent) error
    // ... more event types
}
```

### 2. Filtering Location

Where should we filter events?

**At Subscription:**
```go
bus.On(DamageEvent).
    ToTarget(myID).
    Where(e => e.IsMagical).
    Do(handler)
```

**In Handler:**
```go
func handler(e *DamageEvent) {
    if e.Target != myID { return }
    if !e.IsMagical { return }
    // handle
}
```

### 3. Event Mutation

Should events be mutable (for modifiers) or immutable?

**Mutable (current):**
```go
func handleResistance(e *DamageEvent) {
    e.Damage = e.Damage / 2  // Modify in place
}
```

**Immutable:**
```go
func handleResistance(e *DamageEvent) *DamageEvent {
    return e.WithDamage(e.Damage / 2)  // Return new event
}
```

## Pragmatic First Step

1. **Create typed event structs** for core game events:
   - DamageEvent
   - HealingEvent  
   - SpellCastEvent
   - AttackEvent
   - FeatureActivatedEvent

2. **Keep generic bus** but add type helpers:
   ```go
   // Register typed handler
   bus.OnType(&DamageEvent{}, func(e Event) error {
       dmg := e.(*DamageEvent)  // Safe - bus guarantees type
       return handleDamage(dmg)
   })
   ```

3. **Add safe context getters** as fallback:
   ```go
   damage, err := event.Context().GetInt("damage")
   if err != nil {
       return fmt.Errorf("invalid damage: %w", err)
   }
   ```

4. **Keep boolean flags for now** - they work, event history is future enhancement

## Success Criteria

The goal is to make event handling:
1. **Type-safe** - No runtime panics from bad casts
2. **Discoverable** - IDE knows what fields events have
3. **Efficient** - Filter early, handle only relevant events
4. **Simple** - Easy to add new event types

## Next Steps

1. Define core event types (ADR?)
2. Enhance event bus with type registration
3. Update smart subscriptions for typed events
4. Migrate features to use typed events
5. Consider event history for state tracking (later)

## The Measure

When implementing Rage's damage resistance:
```go
// Should be this simple
bus.On[DamageEvent]().
    ToTarget(barbarian.ID).
    Where(e => e.IsPhysical()).
    Do(func(e *DamageEvent) {
        e.Damage = e.Damage / 2
    })
```

Not string checking and type assertions.