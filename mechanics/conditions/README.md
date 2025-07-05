# Conditions Module

The conditions module provides a comprehensive system for status effects and conditions in RPG games, with full support for D&D 5e mechanics.

## Features

- **All D&D 5e Conditions**: Complete implementation of the 15 standard conditions
- **Mechanical Effects**: Automatic application of advantage/disadvantage, auto-fails, speed changes, etc.
- **Condition Interactions**: Support for immunity, suppression, and included conditions
- **Exhaustion Levels**: Special handling for the 6-level exhaustion system
- **Event Integration**: Conditions automatically modify rolls and actions through the event system
- **Builder Pattern**: Easy creation of conditions with fluent interface
- **Relationship Management**: Track concentration, auras, and linked conditions

## Quick Start

```go
import (
    "github.com/KirkDiggler/rpg-toolkit/mechanics/conditions"
    "github.com/KirkDiggler/rpg-toolkit/events"
)

// Create event bus and condition manager
bus := events.NewBus()
manager := conditions.NewConditionManager(bus)

// Apply poisoned condition
poisoned, err := conditions.Poisoned().
    WithTarget(target).
    WithSource("giant_spider").
    WithSaveDC(11).
    WithMinutesDuration(1).
    Build()

err = manager.ApplyCondition(poisoned)

// Conditions automatically apply their effects
// When the poisoned creature attacks, they'll have disadvantage
```

## Condition Types

### Standard Conditions

| Condition | Effects |
|-----------|---------|
| **Blinded** | Can't see, auto-fail sight checks, attacks have disadvantage, attacks against have advantage |
| **Charmed** | Can't attack charmer, charmer has advantage on social checks |
| **Deafened** | Can't hear, auto-fail hearing checks |
| **Frightened** | Disadvantage on ability checks and attacks while source in sight, can't move closer |
| **Grappled** | Speed becomes 0 |
| **Incapacitated** | Can't take actions or reactions |
| **Invisible** | Heavily obscured, attacks have advantage, attacks against have disadvantage |
| **Paralyzed** | Incapacitated, can't move/speak, auto-fail STR/DEX saves, attacks against have advantage |
| **Petrified** | Incapacitated, can't move/speak, resistance to all damage, immune to poison/disease |
| **Poisoned** | Disadvantage on attacks and ability checks |
| **Prone** | Disadvantage on attacks, melee attacks against have advantage, ranged have disadvantage |
| **Restrained** | Speed 0, disadvantage on attacks and DEX saves, attacks against have advantage |
| **Stunned** | Incapacitated, can't move, speak falteringly, auto-fail STR/DEX saves |
| **Unconscious** | Incapacitated, prone, drops items, auto-fail STR/DEX saves, attacks are crits in melee |

### Exhaustion

Exhaustion has 6 cumulative levels:

1. Disadvantage on ability checks
2. Speed halved
3. Disadvantage on attacks and saves
4. Hit point maximum halved
5. Speed reduced to 0
6. Death

```go
// Exhaustion management
exhaustionMgr := conditions.NewExhaustionManager(manager)

// Add exhaustion
err = exhaustionMgr.AddExhaustion(character, 2, "forced_march")

// Remove on long rest
err = exhaustionMgr.ApplyExhaustionOnRest(character, "long")

// Check for death
if exhaustionMgr.CheckExhaustionDeath(character) {
    // Character has died from exhaustion
}
```

## Enhanced Conditions

Enhanced conditions provide full mechanical effects:

```go
// Create enhanced condition with all effects
config := conditions.EnhancedConditionConfig{
    ID:            "unique_id",
    ConditionType: conditions.ConditionPoisoned,
    Target:        target,
    Source:        "poison_dart",
    SaveDC:        15,
    Duration:      events.NewRoundsDuration(10),
}

condition, err := conditions.NewEnhancedCondition(config)
```

## Condition Manager

The condition manager handles interactions between conditions:

```go
// Add immunity
manager.AddImmunity(entity, conditions.ConditionPoisoned)

// Check if can apply
canApply, reason := manager.CanApplyCondition(entity, conditions.ConditionCharmed)

// Get all conditions
activeConditions := manager.GetConditions(entity)

// Remove specific condition type
err = manager.RemoveConditionType(entity, conditions.ConditionBlinded)
```

## Builder Pattern

Use the builder for easy condition creation:

```go
// Simple conditions
blinded := conditions.Blinded().
    WithTarget(target).
    WithSource("darkness_spell").
    Build()

// With metadata
charmed := conditions.Charmed().
    WithTarget(target).
    WithSource("charm_person").
    WithCharmer(caster).
    WithSaveDC(14).
    WithConcentration().
    Build()

// Exhaustion
exhausted := conditions.Exhaustion(3).
    WithTarget(target).
    WithSource("extreme_conditions").
    Build()
```

## Event Integration

Conditions automatically modify events:

```go
// Attack with disadvantage from poisoned
attackEvent := events.NewGameEvent(
    events.EventOnAttackRoll,
    poisonedCreature,
    target,
    nil,
)

// Publish event - conditions apply their modifiers
bus.Publish(ctx, attackEvent)

// Check modifiers
for _, mod := range attackEvent.Context().GetModifiers() {
    if mod.Type() == events.ModifierDisadvantage {
        // Poisoned applied disadvantage
    }
}
```

## Relationships

Track condition relationships like concentration:

```go
relationships := conditions.NewRelationshipManager(bus)

// Create concentration relationship
err = relationships.CreateRelationship(
    conditions.RelationshipConcentration,
    caster,
    []conditions.Condition{holdPerson, charmPerson},
    nil,
)

// Break concentration
err = relationships.BreakAllRelationships(caster)
```

## Discord Bot Examples

See the `examples` directory for Discord bot integration:

- Spell implementations
- Combat condition handling
- Condition display formatting
- Save mechanics
- Environmental hazards

```go
// Example: Display conditions in Discord
func getConditionEmbed(character Entity) *discordgo.MessageEmbed {
    conditions := manager.GetConditions(character)
    
    fields := []*discordgo.MessageEmbedField{}
    for _, cond := range conditions {
        fields = append(fields, &discordgo.MessageEmbedField{
            Name:  getConditionIcon(cond) + " " + cond.GetType(),
            Value: getConditionDescription(cond),
        })
    }
    
    return &discordgo.MessageEmbed{
        Title:  "Active Conditions",
        Fields: fields,
    }
}
```

## Design Philosophy

1. **Full Mechanical Support**: All D&D 5e condition effects are implemented
2. **Event-Driven**: Conditions work through the event system, not direct modification
3. **Extensible**: Easy to add custom conditions and effects
4. **Type-Safe**: Use Go's type system to prevent errors
5. **Discord-Ready**: Designed for the needs of the Discord bot

## Future Enhancements

- Duration tracking with automatic expiration
- Condition icons and formatting helpers
- More complex condition relationships
- Custom condition templates
- Saving throw automation