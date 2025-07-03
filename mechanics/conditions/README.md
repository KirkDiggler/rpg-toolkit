# RPG Toolkit Conditions

The conditions module provides a flexible system for managing temporary and permanent status effects using the event system.

## Installation

```bash
go get github.com/KirkDiggler/rpg-toolkit/conditions
```

## Core Concepts

### Conditions
Conditions represent status effects that modify game mechanics. Each condition:
- Has a unique ID and type
- Tracks its source and duration
- Can subscribe to events to apply effects
- Automatically expires based on duration rules

### Durations
Multiple duration types are supported:
- **Permanent**: Never expires
- **Rounds**: Expires after N rounds
- **Turns**: Expires after N turns of a specific entity
- **Until Damaged**: Expires when entity takes damage
- **Event-based**: Expires on specific events with custom logic

### Manager
The condition manager tracks all active conditions and handles:
- Adding/removing conditions
- Duration tracking through events
- Thread-safe access for concurrent games
- Event publication for condition changes

## Usage

### Basic Setup

```go
import (
    "github.com/KirkDiggler/rpg-toolkit/conditions"
    "github.com/KirkDiggler/rpg-toolkit/events"
)

// Create event bus and condition manager
bus := events.NewBus()
manager := conditions.NewEventManager(bus)

// Register entities
player := &Character{ID: "player-1"}
manager.RegisterEntity(player)
```

### Applying Conditions

```go
// Create a condition that lasts 3 rounds
duration := conditions.NewRoundsDuration(3)
condition := conditions.NewCondition(
    "poison-1",           // unique ID
    "poisoned",          // condition type
    "giant_spider_bite", // source
    spider,              // source entity
    duration,
)

// Apply to player
err := manager.Add(player.GetID(), condition)
```

### Checking Conditions

```go
// Check if entity has a specific condition type
if manager.HasCondition(player.GetID(), "poisoned") {
    // Apply poison effects
}

// Get all conditions
conditions := manager.GetAll(player.GetID())
for _, cond := range conditions {
    fmt.Printf("%s: %s\n", cond.Type(), cond.Duration().Description())
}
```

### Custom Conditions

Create custom conditions by extending BaseCondition:

```go
type StunnedCondition struct {
    *conditions.BaseCondition
    subscriptionID string
}

func (c *StunnedCondition) OnApply(bus events.EventBus, target core.Entity) error {
    // Subscribe to action events to prevent them
    c.subscriptionID = bus.SubscribeFunc("before_action", 0, 
        func(ctx context.Context, e events.Event) error {
            if e.Source().GetID() == target.GetID() {
                e.Context().Set("prevented", true)
                e.Context().Set("reason", "stunned")
            }
            return nil
        })
    return nil
}

func (c *StunnedCondition) OnRemove(bus events.EventBus, target core.Entity) error {
    bus.Unsubscribe(c.subscriptionID)
    return nil
}
```

## Example: Poisoned Condition

The included PoisonedCondition shows how to implement disadvantage on attacks:

```go
// Apply poison
poisoned := conditions.NewPoisonedCondition(
    "poison-1",
    "poison_dart_trap",
    nil, // no source entity
    conditions.NewRoundsDuration(5),
)
manager.Add(player.GetID(), poisoned)

// Now when the player attacks, they automatically have disadvantage
// The condition subscribes to attack events and adds the modifier
```

## Duration Examples

### Rounds-based
```go
// Lasts 3 rounds
duration := conditions.NewRoundsDuration(3)
```

### Turn-based
```go
// Lasts 2 of the player's turns
duration := conditions.NewTurnsDuration(2, player.GetID())
```

### Until Damaged
```go
// Lasts until the player takes damage
duration := conditions.NewUntilDamagedDuration(player.GetID())
```

### Custom Event
```go
// Lasts until player makes a successful save
duration := conditions.NewEventDuration("save_made", func(e events.Event) bool {
    if e.Target().GetID() == player.GetID() {
        if success, ok := e.Context().Get("success"); ok {
            return success.(bool)
        }
    }
    return false
})
```

## Integration with Events

The condition system integrates deeply with events:

1. **Duration Tracking**: Automatically checks expiration on relevant events
2. **Effect Application**: Conditions can modify events as they happen
3. **Lifecycle Events**: Publishes events when conditions are applied/removed

### Condition Events

```go
// Subscribe to condition changes
bus.SubscribeFunc(conditions.EventConditionApplied, 100, 
    func(ctx context.Context, e events.Event) error {
        condID, _ := e.Context().Get("condition_id")
        condType, _ := e.Context().Get("condition_type")
        fmt.Printf("Condition applied: %s (%s)\n", condID, condType)
        return nil
    })
```

## Thread Safety

The EventManager is fully thread-safe and can handle concurrent:
- Condition additions/removals
- Duration checks
- Event processing

## Best Practices

1. **Use Event Integration**: Let conditions modify behavior through events rather than direct coupling
2. **Unique IDs**: Always use unique IDs for condition instances
3. **Register Entities**: Register entities before applying conditions for proper event publishing
4. **Clean Up**: Conditions automatically unsubscribe from events when removed
5. **Duration Choice**: Pick the right duration type for your game mechanics

## Testing

The module includes comprehensive tests demonstrating:
- Basic condition management
- Duration expiration
- Event integration
- Thread safety
- Custom condition examples