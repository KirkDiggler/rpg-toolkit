# Proficiency Module

The proficiency module provides infrastructure for entities to be proficient with various skills, tools, weapons, or other game elements. Proficiencies modify game mechanics through the event system.

## Philosophy

This module follows the rpg-toolkit design principles:

- **Infrastructure, Not Implementation**: We provide the tools for proficiencies, not specific game rules
- **Event-Driven**: Proficiencies apply their effects by subscribing to events
- **Generic**: No game-specific logic (no D&D 5e calculations, no weapon categories)
- **Self-Contained**: Each proficiency manages its own behavior

## Core Concepts

### Proficiencies as Entities

Like conditions, proficiencies are first-class entities in the game world:
- They have unique IDs for persistence
- They can be queried and managed
- They survive across sessions

### Event Integration

Proficiencies work by subscribing to relevant events and modifying them:

```go
// Example: Weapon proficiency that modifies attack rolls
weaponProf := proficiency.NewSimpleProficiency(proficiency.SimpleProficiencyConfig{
    ID:      "prof-longsword",
    Type:    "proficiency.weapon",
    Owner:   fighter,
    Subject: "longsword",
    Source:  "fighter-class",
    ApplyFunc: func(p *proficiency.SimpleProficiency, bus events.EventBus) error {
        // Subscribe to attack events
        p.Subscribe(bus, events.EventBeforeAttackRoll, 100, func(ctx context.Context, e events.Event) error {
            // Rulebook-specific logic would check weapon and apply bonus
            // The infrastructure just provides the hook
            return nil
        })
        return nil
    },
})
```

## Usage

### Basic Setup

```go
import (
    "github.com/KirkDiggler/rpg-toolkit/mechanics/proficiency"
    "github.com/KirkDiggler/rpg-toolkit/events"
)

// Create event bus
bus := events.NewBus()

// Apply proficiency
prof.Apply(bus)

// Later, remove it
prof.Remove(bus)
```

### Creating Custom Proficiencies

For complex proficiencies, implement the Proficiency interface directly:

```go
type ExpertiseProficiency struct {
    id      string
    owner   core.Entity
    subject string
    // Custom fields for your game system
}

func (e *ExpertiseProficiency) Apply(bus events.EventBus) error {
    // Subscribe to relevant events
    // Apply expertise bonuses (double proficiency, advantage, etc.)
    return nil
}
```

## Integration with Game Systems

The proficiency module doesn't define:
- What proficiency bonuses are (that's rulebook-specific)
- How proficiencies are gained (class/race/training systems)
- Proficiency prerequisites or restrictions

Instead, game systems use this infrastructure to implement their specific rules:

```go
// In your D&D 5e rulebook module
func createWeaponProficiency(character Entity, weapon string) proficiency.Proficiency {
    return proficiency.NewSimpleProficiency(proficiency.SimpleProficiencyConfig{
        // ... configuration ...
        ApplyFunc: func(p *proficiency.SimpleProficiency, bus events.EventBus) error {
            p.Subscribe(bus, "BeforeAttackRoll", 100, func(ctx context.Context, e events.Event) error {
                // D&D 5e specific: Add proficiency bonus based on level
                level := getCharacterLevel(p.Owner())
                bonus := calculateProficiencyBonus(level) // +2 to +6
                // Apply bonus to the event
                return nil
            })
            return nil
        },
    })
}
```

## Design Patterns

### Self-Reference Pattern

Like conditions, proficiencies receive themselves in handler functions:

```go
ApplyFunc: func(p *proficiency.SimpleProficiency, bus events.EventBus) error {
    // 'p' is the proficiency itself, allowing access to owner, subject, etc.
    p.Subscribe(bus, eventType, priority, handler)
    return nil
}
```

### Subscription Tracking

The SimpleProficiency automatically tracks event subscriptions for cleanup:

```go
// Subscribe helper method tracks the subscription ID
p.Subscribe(bus, eventType, priority, handler)

// Remove() automatically unsubscribes all tracked subscriptions
p.Remove(bus)
```

## Comparison with Conditions

Proficiencies and conditions share similar patterns:
- Both are entities
- Both modify game behavior through events
- Both are self-contained
- Both use config-based construction

The key difference is conceptual:
- **Conditions**: Temporary effects (poisoned, blessed, stunned)
- **Proficiencies**: Permanent capabilities (trained with swords, skilled in athletics)

## Examples

See `simple_test.go` for basic examples of proficiency usage.