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

## Behavioral Interfaces

The package provides composable behavioral interfaces that effects can implement to gain specific capabilities. This allows effects to mix and match behaviors as needed.

### Available Behaviors

#### ConditionalEffect
Effects that only apply under certain conditions.
```go
type ConditionalEffect interface {
    CheckCondition(ctx context.Context, event events.Event) bool
}
```
Examples: Weapon proficiency, Sneak Attack, situational bonuses

#### TemporaryEffect
Effects with limited duration that expire.
```go
type TemporaryEffect interface {
    GetDuration() Duration
    CheckExpiration(ctx context.Context, currentTime time.Time) bool
    OnExpire(bus events.EventBus) error
}
```
Examples: Bless (1 minute), Rage (10 rounds), Mage Armor (8 hours)

#### ResourceConsumer
Effects that consume limited resources when activated.
```go
type ResourceConsumer interface {
    GetResourceRequirements() []ResourceRequirement
    ConsumeResources(ctx context.Context, bus events.EventBus) error
}
```
Examples: Rage (uses charges), Divine Smite (spell slots), Ki abilities

#### DiceModifier
Effects that add dice expressions to rolls (rolled fresh each time).
```go
type DiceModifier interface {
    GetDiceExpression(ctx context.Context, event events.Event) string
    GetModifierType() ModifierType
    ShouldApply(ctx context.Context, event events.Event) bool
}
```
Examples: Bless (+1d4), Bane (-1d4), Sneak Attack damage

#### StackableEffect
Effects that define how they combine with other effects.
```go
type StackableEffect interface {
    GetStackingRule() StackingRule
    CanStackWith(other core.Entity) bool
    Stack(other core.Entity) error
}
```
Examples: Ability damage (adds), Temporary HP (max), most buffs (none)

#### Additional Behaviors
- `SavingThrowEffect` - Effects that allow saves
- `TargetedEffect` - Effects affecting specific entities
- `TriggeredEffect` - Effects that respond to triggers

### Composition Example

```go
// Bless combines multiple behaviors
type BlessEffect struct {
    *effects.Core
    // Implements: TemporaryEffect, DiceModifier, TargetedEffect, StackableEffect
}

// Rage combines different behaviors
type RageEffect struct {
    *effects.Core
    // Implements: ConditionalEffect, TemporaryEffect, ResourceConsumer
}

## Design Philosophy

This package follows the rpg-toolkit philosophy:
- **Infrastructure, not implementation** - We provide tools, not rules
- **Composition over inheritance** - Embed and compose as needed
- **Event-driven** - All behavior modification happens through events
- **Domain clarity** - Effects remain domain-specific (Proficiency, Condition, etc.)