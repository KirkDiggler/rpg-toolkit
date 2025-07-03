# RPG Toolkit Events

The events module provides an event-driven architecture for rpg-toolkit, enabling features to compose without tight coupling.

## Installation

```bash
go get github.com/KirkDiggler/rpg-toolkit/events
```

## Core Concepts

### Events
Events represent things that happen in the game (attacks, damage, status changes, etc.). Each event has:
- **Type**: What kind of event (e.g., "attack_roll", "calculate_damage")
- **Source**: The entity that triggered the event
- **Target**: The entity affected by the event
- **Context**: Event-specific data and modifiers

### Event Bus
The event bus manages subscriptions and publishes events to handlers. Handlers are executed in priority order.

### Modifiers
Modifiers allow features to change game mechanics by adding bonuses, penalties, or other effects during event processing.

## Usage

### Basic Event Publishing

```go
import (
    "context"
    "github.com/KirkDiggler/rpg-toolkit/events"
)

// Create event bus
bus := events.NewBus()

// Create an event
event := events.NewGameEvent(events.EventBeforeAttack, attacker, target)

// Add context data
event.Context().Set("weapon", "longsword")
event.Context().Set("attack_type", "melee")

// Publish the event
err := bus.Publish(context.Background(), event)
```

### Subscribing to Events

```go
// Subscribe with a function
bus.SubscribeFunc(events.EventCalculateDamage, 100, func(ctx context.Context, e events.Event) error {
    // Add rage damage bonus
    if hasRage(e.Source()) {
        e.Context().AddModifier(events.NewModifier(
            "rage",                    // source
            events.ModifierDamageBonus, // type
            2,                         // value
            100,                       // priority
        ))
    }
    return nil
})

// Or implement the Handler interface
type SneakAttackHandler struct{}

func (h *SneakAttackHandler) Handle(ctx context.Context, e events.Event) error {
    if canSneakAttack(e.Source(), e.Target()) {
        damage := rollSneakAttackDice()
        e.Context().AddModifier(events.NewModifier(
            "sneak_attack",
            events.ModifierDamageBonus,
            damage,
            150,
        ))
    }
    return nil
}

func (h *SneakAttackHandler) Priority() int {
    return 150 // Higher priority = executes later
}

bus.Subscribe(events.EventCalculateDamage, &SneakAttackHandler{})
```

### Working with Modifiers

Modifiers now use a typed interface for clean, type-safe processing:

```go
// Create modifiers with different value types
proficiencyMod := events.NewModifier(
    "proficiency",
    events.ModifierAttackBonus,
    events.NewRawValue(2, "proficiency"),
    50,
)

// Dice modifiers are rolled at creation time
blessMod := events.NewModifier(
    "bless",
    events.ModifierAttackBonus,
    events.NewDiceValue(1, 4, "bless"), // Rolls 1d4 immediately
    100,
)

// Add modifiers to event
event.Context().AddModifier(proficiencyMod)
event.Context().AddModifier(blessMod)

// Process modifiers cleanly without type assertions
total := 0
descriptions := []string{}

for _, mod := range event.Context().Modifiers() {
    if mod.Type() == events.ModifierAttackBonus {
        mv := mod.ModifierValue()
        total += mv.GetValue()
        descriptions = append(descriptions, mv.GetDescription())
    }
}

// Output might be: "total: 5, descriptions: [+2 (proficiency), +d4[3]=3 (bless)]"
```

For simple integer modifiers, use the convenience function:

```go
// Simple integer modifier
rageMod := events.NewIntModifier("rage", events.ModifierDamageBonus, 2, 100)
```

## Common Event Types

### Combat Events
- `EventBeforeAttack`: Before attack roll
- `EventAttackRoll`: During attack roll calculation
- `EventCalculateDamage`: During damage calculation
- `EventAfterDamage`: After damage is applied

### Status Events
- `EventStatusApplied`: When a status effect is applied
- `EventStatusRemoved`: When a status effect is removed
- `EventStatusCheck`: When checking if a status should expire

### Turn Events
- `EventTurnStart`: At the start of a turn
- `EventTurnEnd`: At the end of a turn
- `EventRoundStart`: At the start of a round
- `EventRoundEnd`: At the end of a round

## Best Practices

1. **Use Priority Wisely**: Lower priority handlers execute first. Use this to ensure proper ordering.

2. **Keep Handlers Focused**: Each handler should do one thing well.

3. **Handle Errors**: Return errors from handlers to stop event propagation.

4. **Thread Safety**: The event bus is thread-safe, but be careful with shared state in handlers.

5. **Avoid Infinite Loops**: Don't publish events from handlers that would trigger the same handler.

## Example: Implementing Rage

```go
type RageFeature struct {
    bus events.EventBus
}

func (r *RageFeature) Initialize() {
    // Listen for damage calculations
    r.bus.SubscribeFunc(events.EventCalculateDamage, 100, r.handleDamage)
    
    // Listen for damage calculation (for resistance)
    r.bus.SubscribeFunc(events.EventCalculateDamage, 50, r.handleIncomingDamage)
}

func (r *RageFeature) handleDamage(ctx context.Context, e events.Event) error {
    if !r.isRaging(e.Source()) {
        return nil
    }
    
    // Only melee attacks get bonus
    if attackType, ok := e.Context().Get("attack_type"); ok && attackType == "melee" {
        e.Context().AddModifier(events.NewIntModifier(
            "rage",
            events.ModifierDamageBonus,
            2,
            100,
        ))
    }
    
    return nil
}

func (r *RageFeature) handleIncomingDamage(ctx context.Context, e events.Event) error {
    if !r.isRaging(e.Target()) {
        return nil
    }
    
    // Check damage type
    if damageType, ok := e.Context().Get("damage_type"); ok {
        if damageType == "bludgeoning" || damageType == "piercing" || damageType == "slashing" {
            // Halve the damage (resistance)
            e.Context().AddModifier(events.NewModifier(
                "rage_resistance",
                "damage_multiplier",
                0.5,
                200,
            ))
        }
    }
    
    return nil
}
```

## Testing

The events module is designed to be easily testable:

```go
func TestMyFeature(t *testing.T) {
    bus := events.NewBus()
    feature := NewMyFeature(bus)
    
    // Create test event
    event := events.NewGameEvent(events.EventCalculateDamage, mockAttacker, mockTarget)
    
    // Publish event
    err := bus.Publish(context.Background(), event)
    
    // Verify modifiers were added
    mods := event.Context().Modifiers()
    // ... assertions
}
```