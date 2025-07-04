# Effects Infrastructure

The `effects` package provides shared infrastructure for game mechanics that modify behavior through the event system.

## Overview

Many game mechanics follow the same pattern:
- Subscribe to events when applied
- Modify game behavior through those subscriptions
- Unsubscribe when removed
- Track active/inactive state

This package provides reusable components to implement these patterns consistently.

## Core Components

### EffectCore

The `Core` type provides base functionality that can be embedded in domain types:

```go
type SimpleProficiency struct {
    effects.Core  // Embed to get standard behavior
    owner   core.Entity
    subject string
}
```

Features:
- Implements `core.Entity` interface
- Automatic subscription tracking and cleanup
- Apply/Remove lifecycle with custom handlers
- Active/inactive state management

### SubscriptionTracker

Manages event subscriptions for automatic cleanup:

```go
tracker := effects.NewSubscriptionTracker()
tracker.Subscribe(bus, "event.type", 100, handler)
// ... later ...
tracker.UnsubscribeAll(bus) // Clean up all subscriptions
```

## Usage Example

```go
// Create a domain type that embeds Core
type MyEffect struct {
    effects.Core
    target core.Entity
}

// Constructor uses CoreConfig
func NewMyEffect(id, source string, target core.Entity) *MyEffect {
    return &MyEffect{
        Core: *effects.NewCore(effects.CoreConfig{
            ID:     id,
            Type:   "my.effect",
            Source: source,
            ApplyFunc: func(bus events.EventBus) error {
                // Custom apply logic here
                return nil
            },
        }),
        target: target,
    }
}

// Use the embedded Core methods
effect := NewMyEffect("effect-1", "spell", character)
err := effect.Apply(bus)    // Activates and runs ApplyFunc
effect.IsActive()           // true
err = effect.Remove(bus)    // Deactivates and cleans up
```

## Behavioral Interfaces (Coming Soon)

Future additions will include composable behaviors:
- `ConditionalEffect` - Only applies under certain conditions
- `TemporaryEffect` - Has duration/expiration
- `ResourceConsumer` - Uses limited resources
- `PropertyModifier` - Modifies entity properties

## Design Philosophy

This package follows the rpg-toolkit philosophy:
- **Infrastructure, not implementation** - We provide tools, not rules
- **Composition over inheritance** - Embed and compose as needed
- **Event-driven** - All behavior modification happens through events
- **Domain clarity** - Effects remain domain-specific (Proficiency, Condition, etc.)