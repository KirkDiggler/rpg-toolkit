# Resources System

The `resources` package provides simple resource tracking for games. Resources track current/maximum values, counters track simple counts.

## Overview

This package provides two main types:
- **Resource**: Tracks depletable resources with current and maximum values (spell slots, hit points, rage uses)
- **Counter**: Tracks simple counts with optional limits (death saves, attacks made, concentration checks)

Both are managed through a **Pool** for organized storage.

## Core Components

### Resource

Tracks values that can be consumed and restored:

```go
// Create a resource at full capacity
rage := resources.NewResource("rage", 3)

// Use the resource
err := rage.Use(1)  // Returns error if insufficient

// Restore partially or fully
rage.Restore(1)      // Add 1 (capped at maximum)
rage.RestoreToFull() // Back to maximum

// Check state
if rage.IsAvailable() { // Has any remaining?
    // Can use the resource
}
```

### Counter

Tracks simple counts with optional limits:

```go
// Create a counter with a limit
deathSaves := resources.NewCounter("death_saves", 3)

// Increment the count
err := deathSaves.Increment() // Returns error if at limit

// Create unlimited counter
attacks := resources.NewCounter("attacks", 0) // 0 = no limit
attacks.IncrementBy(3) // Can increment indefinitely

// Reset when needed
deathSaves.Reset() // Back to 0
```

### Pool

Manages collections of resources and counters:

```go
pool := resources.NewPool()

// Add resources
pool.AddResource(resources.NewResource("hit_points", 45))
pool.AddResource(resources.NewResource("spell_slots_1", 4))
pool.AddResource(resources.NewResource("rage", 3))

// Add counters
pool.AddCounter(resources.NewCounter("death_saves", 3))
pool.AddCounter(resources.NewCounter("attacks_this_turn", 0))

// Use resources
hp, _ := pool.GetResource("hit_points")
hp.Use(10) // Take damage

// Track counts
attacks, _ := pool.GetCounter("attacks_this_turn")
attacks.Increment()

// Rest operations
pool.RestoreAllResources() // Full restore all resources
pool.ResetAllCounters()     // Reset all counters to 0
```

## Usage Examples

### Character Resources
```go
pool := resources.NewPool()

// Hit points
hp := resources.NewResource("hit_points", 45)
pool.AddResource(hp)

// Spell slots by level
pool.AddResource(resources.NewResource("spell_slots_1", 4))
pool.AddResource(resources.NewResource("spell_slots_2", 3))
pool.AddResource(resources.NewResource("spell_slots_3", 2))

// Class abilities
pool.AddResource(resources.NewResource("rage", 3))
pool.AddResource(resources.NewResource("ki", 5))

// Combat
hp.Use(15) // Take damage
spell1, _ := pool.GetResource("spell_slots_1")
spell1.Use(1) // Cast a spell

// Long rest
pool.RestoreAllResources()
```

### Combat Tracking
```go
pool := resources.NewPool()

// Death saving throws
deathSaves := resources.NewCounter("death_saves", 3)
deathFails := resources.NewCounter("death_fails", 3)
pool.AddCounter(deathSaves)
pool.AddCounter(deathFails)

// Combat stats
attacksThisTurn := resources.NewCounter("attacks_this_turn", 0)
pool.AddCounter(attacksThisTurn)

// Track death saves
deathSaves.Increment() // Success
deathFails.Increment() // Failure

if deathSaves.Count >= 3 {
    // Stabilized!
}
if deathFails.AtLimit() {
    // Character dies
}

// End of turn
attacksThisTurn.Reset()
```

### Ability Cooldowns
```go
// Track ability uses with counters
pool := resources.NewPool()

// Abilities that reset on rest
secondWindUsed := resources.NewCounter("second_wind_used", 1)
actionSurgeUsed := resources.NewCounter("action_surge_used", 1)

pool.AddCounter(secondWindUsed)
pool.AddCounter(actionSurgeUsed)

// Use abilities
if !secondWindUsed.AtLimit() {
    secondWindUsed.Increment()
    // Apply Second Wind healing
}

// Short rest
secondWindUsed.Reset()

// Long rest
pool.ResetAllCounters()
```

## Design Philosophy

This package follows rpg-toolkit principles:
- **Simple and direct**: Just tracks numbers, no complex logic
- **Game-agnostic**: Works for any game system
- **Clear separation**: Resources (current/max) vs Counters (simple counts)
- **No hidden complexity**: What you see is what you get

## API Reference

### Resource
- `NewResource(id string, maximum int) *Resource` - Create at full capacity
- `Use(amount int) error` - Consume resource, error if insufficient
- `Restore(amount int)` - Add to current (capped at maximum)
- `RestoreToFull()` - Set to maximum
- `SetCurrent(value int)` - Set directly (clamped to valid range)
- `SetMaximum(value int)` - Change maximum (adjusts current if needed)
- `IsAvailable() bool` - Has any remaining?
- `IsEmpty() bool` - Completely depleted?
- `IsFull() bool` - At maximum?

### Counter
- `NewCounter(id string, limit int) *Counter` - Create counter (0 = no limit)
- `Increment() error` - Add 1, error if at limit
- `IncrementBy(amount int) error` - Add amount, error if would exceed limit
- `Decrement()` - Subtract 1 (minimum 0)
- `DecrementBy(amount int)` - Subtract amount (minimum 0)
- `Reset()` - Set to 0
- `SetCount(value int) error` - Set directly, error if exceeds limit
- `AtLimit() bool` - At maximum count?
- `IsZero() bool` - Count is 0?
- `HasLimit() bool` - Has a maximum limit?

### Pool
- `NewPool() *Pool` - Create empty pool
- `AddResource(r *Resource)` - Add resource to pool
- `AddCounter(c *Counter)` - Add counter to pool
- `GetResource(id string) (*Resource, bool)` - Retrieve resource by ID
- `GetCounter(id string) (*Counter, bool)` - Retrieve counter by ID
- `RemoveResource(id string)` - Remove resource from pool
- `RemoveCounter(id string)` - Remove counter from pool
- `Clear()` - Remove all resources and counters
- `RestoreAllResources()` - Restore all resources to full
- `ResetAllCounters()` - Reset all counters to zero