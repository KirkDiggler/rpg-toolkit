# D&D 5e Proficiency System

This package demonstrates how to implement the D&D 5e proficiency system using rpg-toolkit's event-driven architecture.

## Overview

The D&D 5e proficiency system provides bonuses to various rolls based on character training:
- **Weapon Proficiency**: Adds proficiency bonus to attack rolls
- **Skill Proficiency**: Adds proficiency bonus to ability checks
- **Saving Throw Proficiency**: Adds proficiency bonus to saving throws
- **Expertise**: Doubles proficiency bonus for specific skills

## Architecture

The system uses rpg-toolkit's event bus to automatically apply proficiency bonuses:

```
Character makes attack roll → Event published → Proficiency handlers check weapon → Bonus added to roll
```

## Key Components

### 1. Proficiency Types (`types.go`)
- Defines D&D 5e proficiency categories
- Weapon categories (simple, martial)
- All 18 skills with their associated abilities
- Six saving throw types
- Proficiency bonus calculation (2 + (level-1)/4)

### 2. Weapon Proficiency (`weapon_proficiency.go`)
- Subscribes to `EventOnAttackRoll` events
- Checks if character is proficient with the weapon
- Handles both specific weapons and categories
- Automatically adds proficiency bonus as a modifier

### 3. Skill Proficiency (`skill_proficiency.go`)
- Subscribes to `EventOnAbilityCheck` events
- Supports expertise (double proficiency)
- Maps skills to their base abilities

### 4. Saving Throw Proficiency (`saving_throw_proficiency.go`)
- Subscribes to `EventOnSavingThrow` events
- Adds proficiency bonus for trained saves

### 5. Manager (`manager.go`)
- Central management of all proficiencies
- Class-specific proficiency sets
- Easy integration with existing systems

### 6. DND Bot Integration (`dndbot_integration.go`)
- Shows how to migrate from DND bot's proficiency system
- Entity wrappers for compatibility
- Preserves existing proficiency data

## Usage Example

```go
// Create event bus and manager
eventBus := events.NewBus()
manager := proficiency.NewManager(eventBus)

// Create a character entity
fighter := &CharacterEntity{id: "fighter-1", level: 5}

// Add class proficiencies
manager.AddClassProficiencies(fighter, "fighter", 5)

// When the fighter attacks with a longsword...
attackEvent := events.NewGameEvent(events.EventOnAttackRoll, fighter, nil)
attackEvent.Context().Set("weapon", "longsword")

// The proficiency system automatically adds +3 bonus (level 5 proficiency)
eventBus.Publish(context.Background(), attackEvent)
```

## Integration with DND Bot

The system is designed to work alongside the existing DND bot character system:

1. **Character Wrapper**: Wraps DND bot characters as toolkit entities
2. **Migration Helper**: Converts existing proficiencies to toolkit system
3. **Event Integration**: Works with the toolkit event bus replacement

```go
// Wrap existing DND bot character
entity := NewCharacterWrapper(dndBotCharacter)

// Migrate proficiencies
err := MigrateDNDBotCharacter(dndBotCharacter, manager)
```

## Benefits

1. **Automatic Application**: No need to manually check proficiencies in combat code
2. **Event-Driven**: Proficiencies work through the event system, keeping combat logic clean
3. **Extensible**: Easy to add new proficiency types or special cases
4. **D&D 5e Accurate**: Implements the exact proficiency bonus progression
5. **Compatible**: Works alongside existing DND bot systems

## Testing

Run the comprehensive test suite:

```bash
go test -v
```

The tests demonstrate:
- Weapon proficiency with categories
- Skill proficiency with expertise
- Saving throw proficiency
- Class-specific proficiency sets

## Next Steps

This proficiency system can be extended with:
- Tool proficiencies
- Language proficiencies
- Armor proficiency with AC calculations
- Jack of All Trades (half proficiency to non-proficient checks)
- Custom proficiency sources (feats, racial bonuses)