# Conditions System

The conditions system provides generic infrastructure for implementing status effects in tabletop RPGs. Games define their own condition types and effects using the provided building blocks.

## Design Philosophy

This is **infrastructure, not implementation**. The toolkit provides:
- Generic condition types and effect frameworks
- Builder patterns for easy condition creation
- Event-driven effect application
- Condition management with immunity/suppression
- Relationship tracking (concentration, auras, etc.)

Games implement their specific rules using these tools.

## Basic Usage

### 1. Define Your Game's Conditions

```go
// Register your game's condition types
conditions.RegisterConditionDefinition(&conditions.ConditionDefinition{
    Type:        conditions.ConditionType("poisoned"),
    Name:        "Poisoned",
    Description: "A poisoned creature has disadvantage on attack rolls and ability checks.",
    Effects: []conditions.ConditionEffect{
        {Type: conditions.EffectDisadvantage, Target: conditions.TargetAttackRolls},
        {Type: conditions.EffectDisadvantage, Target: conditions.TargetAbilityChecks},
    },
})

conditions.RegisterConditionDefinition(&conditions.ConditionDefinition{
    Type:        conditions.ConditionType("unconscious"),
    Name:        "Unconscious", 
    Description: "An unconscious creature is incapacitated and prone.",
    Effects: []conditions.ConditionEffect{
        {Type: conditions.EffectIncapacitated, Target: conditions.TargetActions},
    },
    Includes: []conditions.ConditionType{
        conditions.ConditionType("incapacitated"),
        conditions.ConditionType("prone"),
    },
})
```

### 2. Create Condition Builder Helpers

```go
// Your game can create convenient builder functions
func Poisoned() *conditions.ConditionBuilder {
    return conditions.NewConditionBuilder(conditions.ConditionType("poisoned"))
}

func Unconscious() *conditions.ConditionBuilder {
    return conditions.NewConditionBuilder(conditions.ConditionType("unconscious"))
}
```

### 3. Apply Conditions

```go
bus := events.NewBus()
manager := conditions.NewConditionManager(bus)

// Apply a condition
poisoned, err := Poisoned().
    WithTarget(character).
    WithSource("spider_bite").
    WithSaveDC(13).
    WithMinutesDuration(10).
    Build()

if err := manager.ApplyCondition(poisoned); err != nil {
    log.Fatal(err)
}
```

### 4. Handle Game-Specific Logic

```go
// Your game can add immunity
manager.AddImmunity(paladin, conditions.ConditionType("frightened"))

// Check conditions
if manager.HasCondition(character, conditions.ConditionType("poisoned")) {
    fmt.Println("Character is poisoned!")
}

// Get all conditions for display
activeConditions := manager.GetConditions(character)
for _, cond := range activeConditions {
    fmt.Printf("- %s (from %s)\n", cond.GetType(), cond.Source())
}
```

## Available Effect Types

The toolkit provides these generic effect types that games can use:

- `EffectAdvantage` / `EffectDisadvantage` - Modify dice rolls
- `EffectAutoFail` / `EffectAutoSucceed` - Force roll outcomes
- `EffectSpeedReduction` / `EffectSpeedZero` - Movement effects
- `EffectIncapacitated` / `EffectNoReactions` - Action restrictions
- `EffectResistance` / `EffectVulnerability` - Damage modifications
- `EffectCantSpeak` / `EffectCantHear` / `EffectCantSee` - Sensory effects

## Available Effect Targets

Effects can target different game mechanics:

- `TargetAttackRolls` / `TargetAttacksAgainst` - Combat
- `TargetSavingThrows` / `TargetAbilityChecks` - Dice rolling
- `TargetMovement` / `TargetActions` / `TargetReactions` - Actions
- `TargetSight` / `TargetHearing` / `TargetSpeech` - Senses
- `TargetDamage` - Damage calculations

## Condition Relationships

Conditions can have complex relationships:

```go
// Paralyzed includes Incapacitated
paralyzedDef := &conditions.ConditionDefinition{
    Type: conditions.ConditionType("paralyzed"),
    // ...
    Includes: []conditions.ConditionType{
        conditions.ConditionType("incapacitated"),
    },
}

// Immunity relationships
eldrichKnightDef := &conditions.ConditionDefinition{
    Type: conditions.ConditionType("eldritch_ward"),
    // ...
    Immunities: []conditions.ConditionType{
        conditions.ConditionType("charmed"),
        conditions.ConditionType("frightened"),
    },
}
```

## Event Integration

Conditions automatically integrate with the event system:

```go
// The condition system listens for events and applies effects
bus.Subscribe(events.EventOnAttackRoll, func(ctx context.Context, event events.Event) error {
    // Poisoned conditions automatically add disadvantage
    // Blessed conditions automatically add bonuses
    // etc.
    return nil
})
```

## Examples

For complete examples of how to implement specific game systems:

- **D&D 5e**: See `/examples/dnd5e/conditions/`
- **Custom Systems**: The tests show various usage patterns

## Architecture

The conditions system follows our core principles:

1. **Generic Infrastructure**: Provides building blocks, not rules
2. **Event-Driven**: Integrates seamlessly with the event bus
3. **Entity-Based**: Conditions are entities with full lifecycle management
4. **Composable**: Mix and match effects to create complex conditions

This allows the same infrastructure to support D&D 5e, Pathfinder, custom homebrew systems, or any other RPG ruleset.