# Gap Analysis: D&D Discord Bot to RPG Toolkit Migration

## Overview
This document analyzes the gaps between what the D&D Discord bot currently implements for proficiencies, feats, and spells versus what rpg-toolkit provides, identifying what needs to be added to rpg-toolkit to support the bot's requirements.

## 1. Proficiencies System

### Current Discord Bot Implementation
The bot implements proficiencies with the following structure:

**Core Types:**
```go
// In rulebook/dnd5e/proficiency.go
type ProficiencyType string (armor, weapon, tool, skill, saving throw, language)
type Proficiency struct {
    Key, Name, Description string
    Type ProficiencyType
}

// In character/character_proficiencies.go
- Proficiencies stored as map[ProficiencyType][]*Proficiency
- Methods: AddProficiency, SetProficiencies, HasSavingThrowProficiency, HasSkillProficiency
- Proficiency bonus calculation based on level
- Saving throw and skill check bonuses
```

**Key Features:**
- Multiple proficiency types (armor, weapon, tool, skill, saving throws, languages)
- Proficiency bonus scaling with level
- Integration with ability checks and saving throws
- Choice resolution during character creation
- Deduplication of proficiencies

### Current RPG Toolkit Implementation
```go
// In mechanics/proficiency/proficiency.go
type Proficiency interface {
    core.Entity
    Owner() core.Entity
    Subject() string
    Source() string
    IsActive() bool
    Apply(bus events.EventBus) error
    Remove(bus events.EventBus) error
}
```

**Limitations:**
- No built-in proficiency types/categories
- No proficiency bonus calculation
- No integration with skill/saving throw systems
- Proficiencies are full entities (might be overkill for simple proficiencies)

### Gap Analysis for Proficiencies
**Missing in rpg-toolkit:**
1. **Proficiency Types System** - Need categories for armor, weapon, tool, skill, saving throws, languages
2. **Proficiency Bonus Calculator** - Level-based proficiency bonus calculation
3. **Skill/Save Integration** - Automatic bonus application to rolls
4. **Bulk Proficiency Management** - Setting multiple proficiencies at once
5. **Proficiency Choices** - UI/logic for selecting proficiencies during character creation
6. **Simple Proficiency Storage** - Current entity-based approach may be too heavyweight

## 2. Feats/Features System

### Current Discord Bot Implementation
**Core Types:**
```go
// In rulebook/dnd5e/feature.go
type CharacterFeature struct {
    Key, Name, Description string
    Type FeatureType (racial, class, subrace, feat)
    Level int
    Source string
    Metadata map[string]any
}

// In rulebook/dnd5e/features/
- Specific implementations for features like Rage, Sneak Attack, etc.
- AC calculators for Unarmored Defense
- Passive feature handlers
- Feature activation/deactivation logic
```

**Key Features:**
- Different feature types (racial, class, subclass, feat)
- Level requirements
- Metadata for feature-specific data
- Integration with combat modifiers
- Passive vs active features

### Current RPG Toolkit Implementation
```go
// In mechanics/features/feature.go
type Feature interface {
    Key(), Name(), Description() string
    Type() FeatureType
    Level() int
    Source() string
    IsPassive() bool
    GetTiming() FeatureTiming
    GetModifiers() []events.Modifier
    GetProficiencies() []string
    GetResources() []resources.Resource
    // ... activation methods
}
```

**Strengths:**
- Comprehensive feature interface
- Resource integration
- Event listener support
- Prerequisites system
- Feature registry

### Gap Analysis for Features
**Missing in rpg-toolkit:**
1. **D&D-specific Feature Implementations** - Rage, Sneak Attack, Fighting Styles, etc.
2. **AC Calculation Features** - Unarmored Defense variants
3. **Feature Metadata Storage** - For choices like Fighting Style selection
4. **Integration with Discord Bot's Effect System**

## 3. Spells System

### Current Discord Bot Implementation
**Core Types:**
```go
// In character/spells.go
type SpellList struct {
    KnownSpells []string
    PreparedSpells []string
    Cantrips []string
}

// In rulebook/dnd5e/spell.go
type Spell struct {
    Key, Name string
    Level int
    School, CastingTime, Range, Duration string
    Components []string
    Concentration, Ritual bool
    Description, HigherLevel string
    Classes []string
    Damage *SpellDamage
    DC *SpellDC
    AreaOfEffect *SpellAreaOfEffect
    Targeting *AbilityTargeting
}
```

**Key Features:**
- Spell slots by class and level
- Known vs prepared spells
- Cantrips separate from leveled spells
- Spell damage scaling
- Save DC information
- Area of effect handling
- Concentration tracking
- Ritual casting

### Current RPG Toolkit Implementation
**No dedicated spell system** - Only example implementations using conditions

### Gap Analysis for Spells
**Missing in rpg-toolkit:**
1. **Complete Spell System** - No spell entities, spell lists, or spell management
2. **Spell Slot Management** - Tracking and consuming spell slots
3. **Spell Preparation System** - Known vs prepared spells
4. **Spell Casting Interface** - Resolving spell effects, saves, damage
5. **Spell Components** - Verbal, somatic, material tracking
6. **Concentration Management** - Only one concentration spell at a time
7. **Spell School System** - For features that interact with schools
8. **Spell Targeting System** - Single target, multiple targets, areas
9. **Spell Save DC Calculation** - Based on caster stats
10. **Spell Attack Rolls** - For spells that require attack rolls

## 4. Event Bus Replacement

### Current Discord Bot Implementation
```go
type EventBus struct {
    listeners map[EventType][]EventListener
}
- Subscribe/Unsubscribe methods
- Priority-based execution
- Event cancellation
- Custom event types for D&D mechanics
```

### Current RPG Toolkit Implementation
```go
// In events/bus.go
- Generic event bus with typed events
- Handler registration
- Context support
```

### Gap Analysis for Event Bus
**Needed adaptations:**
1. **Event Type Mapping** - Map Discord bot's EventType enum to toolkit's string-based types
2. **Priority System** - Toolkit uses registration order, bot uses explicit priorities
3. **Event Cancellation** - Need to ensure toolkit supports this
4. **D&D-specific Events** - Define all game events in toolkit format

## 5. Recommended Implementation Order

### Phase 1: Core Infrastructure
1. **Spell System Foundation**
   - Create spell entity types
   - Spell slot management
   - Basic spell casting interface

2. **Enhanced Proficiency System**
   - Add proficiency types/categories
   - Proficiency bonus calculator
   - Skill/save integration helpers

### Phase 2: D&D-Specific Features
1. **Feature Implementations**
   - Port key features (Rage, Sneak Attack, etc.)
   - AC calculation features
   - Fighting style system

2. **Spell Implementations**
   - Common spell effects using conditions
   - Spell targeting system
   - Concentration management

### Phase 3: Event Bus Migration
1. **Event Adapter**
   - Create adapter to translate between bot and toolkit events
   - Implement priority handling wrapper
   - Test with existing bot features

### Phase 4: Character Creation Support
1. **Choice Resolution**
   - Proficiency choice UI helpers
   - Feature selection interfaces
   - Spell selection for casters

## 6. API Additions Needed

### For Spells Package
```go
package spells

type SpellSchool string
type ComponentType string

type Spell interface {
    core.Entity
    Level() int
    School() SpellSchool
    Components() []ComponentType
    // ... rest of spell interface
}

type SpellCaster interface {
    GetSpellSlots(level int) int
    GetSpellSaveDC() int
    GetSpellAttackBonus() int
    // ... spell management methods
}

type SpellList interface {
    AddKnownSpell(spell Spell) error
    PrepareSpell(spell Spell) error
    GetPreparedSpells() []Spell
    // ... list management
}
```

### For Enhanced Proficiencies
```go
package proficiency

type ProficiencyType string
type ProficiencyCategory string

type SimpleProficiency struct {
    Type     ProficiencyType
    Category ProficiencyCategory
    Subject  string
    Source   string
}

type ProficiencyHolder interface {
    AddProficiency(prof SimpleProficiency) error
    GetProficiencyBonus() int
    IsProficient(category ProficiencyCategory, subject string) bool
}
```

### For D&D Features
```go
package dnd5e

type ACCalculator interface {
    CalculateAC(character core.Entity) int
}

type FightingStyle interface {
    features.Feature
    GetCombatModifiers() []events.Modifier
}
```

## 7. Migration Strategy

1. **Start with new spell system** - It's completely missing and needed
2. **Enhance proficiency system** - Add D&D-specific needs while keeping generic
3. **Create feature library** - Port Discord bot features to toolkit patterns
4. **Build event adapter** - Allow gradual migration from bot's event bus
5. **Test with character creation flow** - Ensure all pieces work together

## 8. Backwards Compatibility Concerns

- Keep existing toolkit interfaces intact
- Add D&D-specific packages rather than modifying core
- Use composition and adapters for integration
- Maintain generic nature of toolkit while supporting D&D needs