# Event Bus Migration Guide

This guide shows how to replace the DND bot's event system with rpg-toolkit's event bus.

## Overview

The migration can be done gradually:
1. Replace the event bus with an adapter
2. Existing handlers continue to work
3. New code uses toolkit features
4. Gradually update old handlers

## Step 1: Create Event Bus Adapter

In `internal/adapters/toolkit/event_bus.go`:

```go
package toolkit

import (
    "context"
    "github.com/KirkDiggler/rpg-toolkit/events"
    "github.com/KirkDiggler/dnd-bot-discord/internal/domain/game"
)

// EventBusAdapter wraps toolkit's event bus for compatibility
type EventBusAdapter struct {
    bus *events.Bus
    
    // Keep track of old-style subscriptions for cleanup
    subscriptions map[string]string
}

func NewEventBusAdapter() *EventBusAdapter {
    return &EventBusAdapter{
        bus: events.NewBus(),
        subscriptions: make(map[string]string),
    }
}

// Subscribe maintains compatibility with old event system
func (a *EventBusAdapter) Subscribe(eventType game.EventType, listener game.EventListener) {
    // Convert to toolkit handler
    handler := func(ctx context.Context, e events.Event) error {
        // Extract data from toolkit event
        oldEvent := convertToOldEvent(e)
        listener(oldEvent)
        return nil
    }
    
    // Map event type
    toolkitEvent := mapEventType(eventType)
    
    // Subscribe with priority from listener
    id := a.bus.SubscribeFunc(toolkitEvent, listener.Priority(), handler)
    a.subscriptions[string(eventType)] = id
}
```

## Step 2: Replace EventBus in Services

Update service initialization:

```go
// Before:
type CombatService struct {
    eventBus *game.EventBus
}

// After:
type CombatService struct {
    eventBus *toolkit.EventBusAdapter
}

func NewCombatService(bus *toolkit.EventBusAdapter) *CombatService {
    return &CombatService{
        eventBus: bus,
    }
}
```

## Step 3: Event Type Mapping

Map DND bot events to toolkit events:

```go
func mapEventType(oldType game.EventType) string {
    switch oldType {
    case game.EventTypeOnAttackRoll:
        return events.EventOnAttackRoll
    case game.EventTypeOnDamageRoll:
        return events.EventOnDamageRoll
    case game.EventTypeOnSavingThrow:
        return events.EventOnSavingThrow
    // ... etc
    default:
        return string(oldType)
    }
}
```

## Step 4: Gradual Handler Migration

### Old Style (Still Works)
```go
eventBus.Subscribe(game.EventTypeOnAttackRoll, &AttackListener{
    Priority: 100,
    Handler: func(e game.Event) {
        data := e.Data.(*game.AttackEventData)
        // Add modifiers
        data.AttackBonus += 2
    },
})
```

### New Style (Toolkit Features)
```go
bus.GetToolkitBus().SubscribeFunc(events.EventOnAttackRoll, 100,
    func(ctx context.Context, e events.Event) error {
        // Use modifier system
        e.Context().AddModifier(events.NewModifier(
            "proficiency",
            events.ModifierAttackBonus,
            events.NewRawValue(2, "proficiency"),
            100, // priority
        ))
        return nil
    })
```

## Step 5: Benefits of Migration

### 1. Richer Event Context
```go
// Old: Limited data passing
type AttackEventData struct {
    Attacker    string
    Target      string
    AttackBonus int
}

// New: Full context with modifiers
e.Context().Set("weapon", weapon)
e.Context().Set("has_advantage", true)
e.Context().AddModifier(blessModifier)
```

### 2. Modifier System
```go
// Old: Direct manipulation
data.AttackBonus += profBonus + magicBonus + blessBonus

// New: Trackable modifiers
e.Context().AddModifier(proficiencyMod)
e.Context().AddModifier(magicWeaponMod)
e.Context().AddModifier(blessMod)

// Can see all modifiers
for _, mod := range e.Context().Modifiers() {
    log.Printf("%s: +%d", mod.Source(), mod.Value())
}
```

### 3. Event Cancellation
```go
// Counterspell can cancel spell cast
bus.SubscribeFunc(events.EventOnSpellCast, 100, func(ctx context.Context, e events.Event) error {
    if shouldCounterspell(e) {
        e.Cancel()
        return nil
    }
    return nil
})
```

### 4. Priority System
```go
// Handlers execute in priority order
bus.SubscribeFunc("attack.roll", 100, proficiencyHandler)  // First
bus.SubscribeFunc("attack.roll", 90, magicItemHandler)     // Second  
bus.SubscribeFunc("attack.roll", 80, conditionHandler)     // Third
```

## Migration Checklist

- [ ] Create EventBusAdapter
- [ ] Replace EventBus in main.go
- [ ] Update service constructors
- [ ] Test existing handlers still work
- [ ] Start using toolkit events for new features
- [ ] Gradually update old handlers
- [ ] Remove adapter when migration complete

## Common Patterns

### Attack Roll with Modifiers
```go
bus.SubscribeFunc(events.EventOnAttackRoll, 100, func(ctx context.Context, e events.Event) error {
    char := e.Source().(*CharacterEntity)
    weapon, _ := e.Context().GetString("weapon")
    
    // Proficiency bonus
    if profService.CheckProficiency(char.GetID(), weapon) {
        bonus := profService.GetProficiencyBonus(char.Level)
        e.Context().AddModifier(events.NewModifier(
            "proficiency", events.ModifierAttackBonus, 
            events.NewRawValue(bonus, "proficiency"), 100))
    }
    
    // Magic weapon bonus
    if weapon.IsMagic() {
        e.Context().AddModifier(events.NewModifier(
            weapon.Name, events.ModifierAttackBonus,
            events.NewRawValue(weapon.Bonus, weapon.Name), 90))
    }
    
    return nil
})
```

### Damage Reduction
```go
bus.SubscribeFunc(events.EventBeforeTakeDamage, 100, func(ctx context.Context, e events.Event) error {
    target := e.Target()
    damageType, _ := e.Context().GetString("damage_type")
    
    // Check resistances
    if hasResistance(target, damageType) {
        e.Context().AddModifier(events.NewModifier(
            "resistance", events.ModifierDamageReduction,
            events.NewMultiplier(0.5, "resistance"), 50))
    }
    
    return nil
})
```

## Testing the Migration

```go
func TestEventBusCompatibility(t *testing.T) {
    adapter := NewEventBusAdapter()
    
    // Old style should still work
    called := false
    adapter.Subscribe(game.EventTypeOnAttackRoll, &MockListener{
        Handler: func(e game.Event) {
            called = true
        },
    })
    
    // Publish old style
    adapter.Publish(game.EventTypeOnAttackRoll, &game.AttackEventData{})
    
    assert.True(t, called, "Old style handler should be called")
}
```