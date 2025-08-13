# Events Package - Type-Safe Event Bus

## Philosophy

Events are just data. The bus is just plumbing. Events know their refs. ~300 lines total.

## Core Design

Events carry their own ref, eliminating the need to pass refs when publishing. A single package-level ref is shared by both the event and its TypedRef, ensuring perfect consistency.

## Complete Example: Combat System

### 1. Define Your Events (combat/events.go)

```go
package combat

import (
    "github.com/KirkDiggler/rpg-toolkit/core"
    "github.com/KirkDiggler/rpg-toolkit/events"
)

// Single source of truth - package-level ref
var damageEventRef = func() *core.Ref {
    r, _ := core.ParseString("combat:event:damage")
    return r
}()

// DamageType constants
type DamageType string

const (
    DamageTypeSlashing DamageType = "slashing"
    DamageTypePiercing DamageType = "piercing"
    DamageTypeFire     DamageType = "fire"
)

// DamageEvent is just data
type DamageEvent struct {
    Source     core.Entity
    Target     core.Entity
    Amount     int
    DamageType DamageType
}

// EventRef returns THE ref (not a copy, THE actual ref)
func (e *DamageEvent) EventRef() *core.Ref {
    return damageEventRef  // Same pointer every time!
}

// DamageEventRef for type-safe subscriptions
var DamageEventRef = &core.TypedRef[*DamageEvent]{
    Ref: damageEventRef,  // Same ref object!
}
```

### 2. Subscribe to Events (character/character.go)

```go
package character

import (
    "github.com/KirkDiggler/rpg-toolkit/combat"
    "github.com/KirkDiggler/rpg-toolkit/events"
)

type Character struct {
    ID         string
    HP         int
    MaxHP      int
    eventBus   events.EventBus
    eventSubs  []string
}

// SetupEventHandlers subscribes to relevant events
func (c *Character) SetupEventHandlers() error {
    // Simple subscription - gets ALL damage events
    id1, err := events.Subscribe(
        c.eventBus,
        combat.DamageEventRef,
        c.handleAnyDamage,
    )
    if err != nil {
        return err
    }
    c.eventSubs = append(c.eventSubs, id1)
    
    // Filtered subscription - only MY damage
    id2, err := events.Subscribe(
        c.eventBus,
        combat.DamageEventRef,
        c.handleMyDamage,
        events.Where(func(e *combat.DamageEvent) bool {
            return e.Target == c
        }),
    )
    if err != nil {
        return err
    }
    c.eventSubs = append(c.eventSubs, id2)
    
    return nil
}

// Handler receives typed events
func (c *Character) handleMyDamage(e *combat.DamageEvent) error {
    c.HP -= e.Amount
    
    // Check for resistance
    if c.hasResistance(e.DamageType) {
        c.HP += e.Amount / 2 // Take half damage
    }
    
    if c.HP <= 0 {
        c.HP = 0
        // Publish character downed event...
    }
    
    return nil
}

// Cleanup unsubscribes from all events
func (c *Character) Cleanup() {
    for _, id := range c.eventSubs {
        c.eventBus.Unsubscribe(id)
    }
}
```

### 3. Publish Events (combat/combat.go)

```go
package combat

import "github.com/KirkDiggler/rpg-toolkit/events"

type CombatSystem struct {
    bus events.EventBus
}

// ResolveAttack calculates and publishes damage
func (cs *CombatSystem) ResolveAttack(attacker, defender core.Entity, weapon Weapon) error {
    // Calculate damage
    damage := cs.calculateDamage(weapon, attacker)
    
    // Publish - no ref parameter needed!
    return events.Publish(cs.bus, &DamageEvent{
        Source:     attacker,
        Target:     defender,
        Amount:     damage,
        DamageType: weapon.DamageType(),
    })
}
```

## Key Design Features

### Single Source of Truth

Each event type has ONE ref defined at package level:

```go
// Package-level ref - shared by everything
var damageEventRef = func() *core.Ref {
    r, _ := core.ParseString("combat:event:damage")
    return r
}()

// Event returns it
func (e *DamageEvent) EventRef() *core.Ref {
    return damageEventRef  // Same object!
}

// TypedRef uses it
var DamageEventRef = &core.TypedRef[*DamageEvent]{
    Ref: damageEventRef,  // Same object!
}
```

### Runtime Validation

Subscribe validates that events use the correct ref via pointer comparison:

```go
if event.EventRef() != typedRef.Ref {
    // Bug detected - event and TypedRef not using same ref
}
```

### Duplicate Detection

The system detects if multiple event types use the same ref string:

```go
// ErrDuplicateRef shows which types conflict
"duplicate event ref \"combat:event:damage\": 
 already registered by *combat.DamageEvent, 
 attempted by *other.DamageEvent"
```

## API Summary

### Core Types

```go
// Events must implement RefEvent
type RefEvent interface {
    EventRef() *core.Ref
}

// Type aliases for clarity
type EventHandler[T any] = func(T) error
type EventFilter[T any] = func(T) bool
```

### Publishing

```go
// Event knows its ref - no parameter needed!
events.Publish(bus, &DamageEvent{
    Source: attacker,
    Target: defender,
    Amount: 10,
    DamageType: DamageTypeSlashing,
})
```

### Subscribing

```go
// Simple subscription
id, err := events.Subscribe(bus, ref, handler)

// With filter (variadic options)
id, err := events.Subscribe(
    bus,
    ref,
    handler,
    events.Where(filter),
)
```

## Why This Design Works

1. **Clean API** - `Publish(bus, event)` without ref parameter
2. **Type Safety** - TypedRef ensures compile-time type checking
3. **Runtime Validation** - Pointer comparison catches bugs
4. **Single Source of Truth** - One ref per event type
5. **No Noise** - Filters at bus level, handlers only see relevant events
6. **Duplicate Detection** - Registry catches ref string conflicts

## Implementation Stats

- **~300 lines** total (bus.go + typed.go + errors.go)
- **34ns per publish** with single handler
- **Type safe** at compile time
- **Runtime validated** via pointer comparison

## Best Practices

1. **Define ref once** - Package-level variable
2. **Share the ref** - Event.EventRef() and TypedRef.Ref use same object
3. **Store subscription IDs** - For proper cleanup
4. **Filter early** - Use Where() to reduce noise
5. **Events are immutable** - Unless explicitly designed for modification

## That's It

No complex hierarchies. No priority systems. Just:

```go
Publish(bus, event) → Subscribe(bus, ref, handler, Where(filter)) → handler(event)
```

The ref ties everything together - compile-time safe, runtime validated, simple to use.