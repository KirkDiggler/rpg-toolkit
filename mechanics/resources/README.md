# Resources System

The `resources` package provides infrastructure for managing consumable game resources such as spell slots, ability uses, hit dice, and action economy.

## Overview

Resources are consumable values that track current and maximum amounts, with rules for consumption and restoration. This package provides generic infrastructure that game systems can use to implement their specific resource mechanics.

## Core Components

### Resource Interface

The base interface for all resources:

```go
type Resource interface {
    core.Entity
    Owner() core.Entity
    Key() string
    Current() int
    Maximum() int
    Consume(amount int) error
    Restore(amount int)
    RestoreOnShortRest() int
    RestoreOnLongRest() int
    IsAvailable() bool
}
```

### SimpleResource

Basic implementation with configurable restoration:

```go
resource := resources.NewSimpleResource(resources.SimpleResourceConfig{
    ID:       "rage-uses",
    Type:     resources.ResourceTypeAbilityUse,
    Owner:    barbarian,
    Key:      "rage_uses",
    Current:  3,
    Maximum:  3,
    RestoreType: resources.RestoreLongRest,
    LongRestRestore: -1, // Full restore
})
```

### Resource Pool

Manages collections of resources for an entity:

```go
pool := resources.NewSimplePool(character)

// Add resources
pool.Add(resources.CreateSpellSlots(character, map[int]int{
    1: 4,  // 4 first level slots
    2: 3,  // 3 second level slots
    3: 2,  // 2 third level slots
}))

// Consume resources
err := pool.Consume("spell_slots_1", 1, eventBus)

// Process rests
pool.ProcessShortRest(eventBus)
pool.ProcessLongRest(eventBus)
```

## Resource Types

- **Spell Slots**: Level-based spell casting resources
- **Ability Uses**: Limited-use class features
- **Hit Dice**: Rest and recovery resources
- **Action Economy**: Actions, bonus actions, reactions
- **Custom**: Any game-specific resource

## Restoration Types

- **Never**: Resource doesn't automatically restore
- **Turn**: Restores at start of turn
- **Short Rest**: Restores on short rest
- **Long Rest**: Restores on long rest
- **Custom**: Game-specific restoration rules

## Helper Functions

### Spell Slots
```go
slots := resources.CreateSpellSlots(wizard, map[int]int{
    1: 4,
    2: 3,
    3: 2,
})
```

### Ability Uses
```go
rage := resources.CreateAbilityUse(barbarian, "rage", 3, resources.RestoreLongRest)
secondWind := resources.CreateAbilityUse(fighter, "second_wind", 1, resources.RestoreShortRest)
```

### Hit Dice
```go
hitDice := resources.CreateHitDice(fighter, "d10", 10) // 10d10 hit dice
```

### Action Economy
```go
actions := resources.CreateActionEconomy(character)
// Creates action, bonus_action, and reaction resources
```

## Event Integration

Resources publish events when consumed or restored:

```go
// Listen for resource consumption
bus.Subscribe(resources.EventResourceConsumed, func(e events.Event) error {
    event := e.(*resources.ResourceConsumedEvent)
    fmt.Printf("%s consumed %d %s\n", 
        event.Source().GetID(), 
        event.Amount, 
        event.Resource.Key())
    return nil
})
```

## Usage Examples

### Wizard Spell Management
```go
wizard := &Character{id: "wizard-1"}
pool := resources.NewSimplePool(wizard)

// Add spell slots
for _, slot := range resources.CreateSpellSlots(wizard, map[int]int{
    1: 4, 2: 3, 3: 2,
}) {
    pool.Add(slot)
}

// Cast a spell
err := pool.ConsumeSpellSlot(2, bus) // Cast 2nd level spell

// Long rest restores all slots
pool.ProcessLongRest(bus)
```

### Fighter Abilities
```go
fighter := &Character{id: "fighter-1"}
pool := resources.NewSimplePool(fighter)

// Add abilities
pool.Add(resources.CreateAbilityUse(fighter, "second_wind", 1, resources.RestoreShortRest))
pool.Add(resources.CreateAbilityUse(fighter, "action_surge", 1, resources.RestoreLongRest))
pool.Add(resources.CreateHitDice(fighter, "d10", 10))

// Use Second Wind
pool.Consume("second_wind_uses", 1, bus)

// Short rest restores Second Wind
pool.ProcessShortRest(bus)
```

### Combat Action Economy
```go
// At start of combat, create action resources
for _, action := range resources.CreateActionEconomy(character) {
    pool.Add(action)
}

// Use action
pool.Consume("action", 1, bus)

// Actions restore at start of turn (handled by combat system)
```

## Design Philosophy

This package follows rpg-toolkit principles:
- **Infrastructure, not rules**: We provide resource tracking, games define what resources mean
- **Event-driven**: Resources interact through events
- **Entity-based**: Resources are entities for persistence
- **Flexible restoration**: Games define when and how resources restore