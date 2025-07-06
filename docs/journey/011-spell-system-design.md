# Spell System Design

Date: 2025-01-05

## Goal

Design a flexible spell system for rpg-toolkit that can support D&D 5e's requirements while remaining generic enough for other systems.

## Key Requirements

From the Discord bot analysis:
1. Spell entities with components, ranges, durations
2. Spell slot management by class and level
3. Known vs prepared spell tracking
4. Spell casting with saves and attacks
5. Concentration management
6. Ritual and reaction casting
7. Damage/healing with scaling
8. Area of effect handling

## Design Principles

1. **Generic Core**: Base spell interface that any system can implement
2. **Event-Driven**: Spell casting triggers events for modifiers
3. **Resource Integration**: Spell slots as resources
4. **Condition Integration**: Concentration as a condition
5. **Extensible**: Easy to add new spell effects

## Core Architecture

### 1. Spell Interface

```go
type Spell interface {
    core.Entity
    
    // Basic Properties
    Level() int
    School() string
    CastingTime() time.Duration
    Range() int
    Duration() events.Duration
    
    // Components
    RequiresVerbal() bool
    RequiresSomatic() bool
    RequiresMaterial() bool
    MaterialComponent() string
    
    // Casting Properties
    IsRitual() bool
    RequiresConcentration() bool
    
    // Targeting
    TargetType() TargetType // Self, Single, Area, etc.
    AreaOfEffect() AreaOfEffect
    
    // Effects
    Cast(caster core.Entity, targets []core.Entity, slot int, bus events.EventBus) error
}
```

### 2. Spell Slots as Resources

```go
// Use existing resource system
type SpellSlotPool struct {
    resources.Pool
    class string
}

// Track slots per level
func (p *SpellSlotPool) GetSlotsForLevel(level int) int
func (p *SpellSlotPool) UseSlot(level int) error
func (p *SpellSlotPool) RestoreSlots(trigger string)
```

### 3. Spell List Management

```go
type SpellList interface {
    // Known spells (sorcerer, bard, ranger)
    AddKnownSpell(spell Spell) error
    RemoveKnownSpell(spellID string) error
    GetKnownSpells() []Spell
    
    // Prepared spells (wizard, cleric, druid)
    PrepareSpell(spell Spell) error
    UnprepareSpell(spellID string) error
    GetPreparedSpells() []Spell
    IsPrepared(spellID string) bool
    
    // Cantrips
    AddCantrip(spell Spell) error
    GetCantrips() []Spell
}
```

### 4. Concentration Tracking

```go
// Leverage existing condition system
type ConcentrationCondition struct {
    conditions.SimpleCondition
    spellID string
    saveDC int
}

// Break concentration on damage
func (c *ConcentrationCondition) HandleDamage(damage int) bool {
    dc := max(10, damage/2)
    // Trigger Constitution save event
}
```

### 5. Spell Casting Events

```go
// New event types
const (
    EventSpellCastStart = "spell.cast.start"
    EventSpellCastComplete = "spell.cast.complete"
    EventSpellSave = "spell.save"
    EventSpellAttack = "spell.attack"
    EventConcentrationCheck = "spell.concentration.check"
)

type SpellCastEvent struct {
    Caster core.Entity
    Spell Spell
    Targets []core.Entity
    SlotLevel int
}
```

## Implementation Strategy

### Phase 1: Core Spell System
1. Create `mechanics/spells` module
2. Define Spell interface and basic types
3. Implement SimpleSpell for basic spells
4. Add spell casting events

### Phase 2: Spell Slots
1. Extend resource system for spell slots
2. Add slot tracking by class/level
3. Implement rest-based restoration

### Phase 3: Spell Lists
1. Create spell list management
2. Support known vs prepared paradigms
3. Add cantrip support

### Phase 4: Advanced Features
1. Concentration as a condition
2. Ritual casting support
3. Reaction spell framework
4. Metamagic hooks

## Example Usage

```go
// Create a spell
fireball := spells.NewDamageSpell(spells.SpellConfig{
    ID: "fireball",
    Name: "Fireball",
    Level: 3,
    School: "evocation",
    Range: 150,
    AreaOfEffect: spells.Sphere(20),
    Damage: dice.D(6, 8), // 8d6
    DamageType: "fire",
    SaveType: "dexterity",
})

// Cast it
err := fireball.Cast(wizard, targets, 3, bus)

// Handle concentration
shield := spells.NewConcentrationSpell(...)
concentrating := conditions.NewConcentrationCondition(shield)
manager.ApplyCondition(concentrating)
```

## Integration Points

1. **Events**: Spell casting triggers modifier events
2. **Resources**: Spell slots consume resources
3. **Conditions**: Concentration uses condition system
4. **Dice**: Damage rolls use dice module
5. **Features**: Some features modify spell casting

## Open Questions

1. How to handle spell components inventory?
2. Should we support custom spell effects via functions?
3. How granular should spell events be?
4. Should area of effect be part of core or game-specific?

## Next Steps

1. Create module structure
2. Define core interfaces
3. Implement basic spell types
4. Add D&D-specific spells
5. Create examples