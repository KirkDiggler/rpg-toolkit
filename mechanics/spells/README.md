# Spell System Module

The spell system provides a flexible framework for implementing magic systems in RPGs.

## Features

- **Spell Interface**: Generic spell abstraction that any system can implement
- **Spell Slots**: Resource-based slot management with different progression tables
- **Spell Lists**: Support for both "known" and "prepared" spell paradigms  
- **Concentration**: Integration with condition system for concentration tracking
- **Event-Driven**: Spell casting publishes events for game systems to react to
- **Flexible Targeting**: Support for single target, area effects, and more

## Core Components

### Spell Interface

The `Spell` interface defines the contract for all spells:

```go
type Spell interface {
    core.Entity
    
    // Properties
    Level() int
    School() string
    CastingTime() time.Duration
    Range() int
    Duration() events.Duration
    
    // Components
    Components() CastingComponents
    
    // Casting
    Cast(context CastContext) error
}
```

### Spell Slots

Spell slots are managed as resources with automatic restoration:

```go
// Define your game's spell slot progression
table := &YourGameSpellSlotTable{} // Implements SpellSlotTable

// Create spell slots for a caster
owner := wizard // The entity that owns these slots
slots := NewSpellSlotPool(owner, "wizard", 5, table)

// Check and use slots
if slots.HasSlot(3) {
    err := slots.UseSlot(3, bus) // Use a 3rd level slot
}

// Restore on rest
slots.RestoreSlots("long_rest", bus)
```

### Spell Lists

Support for different spell preparation paradigms:

```go
// Known caster (Sorcerer)
knownList := NewSimpleSpellList(SpellListConfig{
    PreparationStyle: "known",
})

// Prepared caster (Wizard)
preparedList := NewSimpleSpellList(SpellListConfig{
    PreparationStyle: "prepared",
    MaxPreparedSpells: 10,
})
```

### Concentration

Concentration integrates with the condition system:

```go
concentrationMgr := NewConcentrationManager(conditionMgr)

// Start concentrating
err := concentrationMgr.StartConcentrating(wizard, spell, saveDC)

// Check concentration
if concentrationMgr.IsConcentrating(wizard) {
    // Handle concentration limits
}
```

## Usage Example

```go
// Create a damage spell
fireball := NewSimpleSpell(SimpleSpellConfig{
    ID:          "fireball",
    Name:        "Fireball",
    Level:       3,
    School:      "evocation",
    Range:       150,
    TargetType:  TargetArea,
    AreaOfEffect: &AreaOfEffect{
        Shape: AreaSphere,
        Size:  20,
    },
    CastFunc: func(ctx CastContext) error {
        // Spell implementation
        for _, target := range ctx.Targets {
            // Roll damage, save, apply effects
        }
        return nil
    },
})

// Add to spell list
spellList.AddKnownSpell(fireball)

// Cast the spell
if spellList.CanCast("fireball") && slots.HasSlot(3) {
    context := CastContext{
        Caster:    wizard,
        Targets:   enemies,
        SlotLevel: 3,
        Bus:       eventBus,
    }
    
    if err := fireball.Cast(context); err == nil {
        slots.UseSlot(3)
    }
}
```

## Events

The spell system publishes several events:

- `spell.cast.attempt` - When casting begins
- `spell.cast.start` - After validation passes
- `spell.cast.complete` - On successful cast
- `spell.cast.failed` - On failed cast
- `spell.save` - When a save is required
- `spell.attack` - For spell attacks
- `spell.damage` - When damage is dealt
- `spell.concentration.check` - When concentration is tested
- `spell.concentration.broken` - When concentration ends

## Extending the System

### Custom Spell Types

Create specialized spell implementations:

```go
type HealingSpell struct {
    *SimpleSpell
    baseHealing dice.Dice
}

type SummonSpell struct {
    *SimpleSpell
    creature Monster
    duration time.Duration
}
```

### Rulebook-Specific Configuration

The spell system intentionally excludes game-specific rules like spell slot progressions. These should be implemented in your game module:

```go
// In your game module (e.g., dnd5e)
type FullCasterTable struct {
    // D&D 5e specific progression
}

func (t *FullCasterTable) GetSlots(level, spellLevel int) int {
    // Your game's spell slot progression
}
```

This keeps the core spell system generic and reusable across different RPG systems.

### Spell Components

The system supports component tracking:

```go
components := CastingComponents{
    Verbal:   true,
    Somatic:  true,
    Material: true,
    Materials: "ruby dust worth 50gp",
    Consumed: true,
    Cost:     50,
}
```

## Design Decisions

1. **Resource Integration**: Spell slots use the existing resource system for consistency
2. **Condition Integration**: Concentration uses conditions to leverage existing mechanics
3. **Event-Driven**: All spell activities publish events for maximum flexibility
4. **Generic Core**: The base system makes no assumptions about specific game rules

## Future Enhancements

- Metamagic support through event modifiers
- Spell component inventory integration  
- Reaction spell framework
- Spell effect templates (damage, healing, buff, debuff)
- Area of effect calculations with positioning