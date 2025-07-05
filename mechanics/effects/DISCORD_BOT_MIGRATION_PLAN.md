# Discord Bot to RPG Toolkit Migration Plan

## Overview
This document provides a comprehensive plan for migrating game mechanics from the `dnd-bot-discord` implementation to the modular `rpg-toolkit` library.

## Executive Summary
The Discord bot has a mature implementation of D&D 5e mechanics including:
- Complete character system with ability scores, proficiencies, and equipment
- All 15 standard D&D conditions with mechanical effects
- Comprehensive effects system with duration tracking and modifiers
- Event-driven architecture for extensibility
- Class features for all base classes (Barbarian, Fighter, Rogue, etc.)
- Combat system with initiative, turn order, and action economy
- Resource management (HP, spell slots, ability uses)
- **In-progress**: Fighting styles and spell system implementation

The bot was actively implementing fighting styles (Great Weapon Fighting with die reroll tracking) and spells (Magic Missile, Vicious Mockery) when the decision was made to migrate logic to the toolkit.

## Current Discord Bot Implementation Analysis

### 1. Core Game Mechanics Currently Implemented

#### Character System
- **Character Model**: Comprehensive character representation with:
  - Ability scores (STR, DEX, CON, INT, WIS, CHA)
  - Race, Class, Background
  - Hit points, AC, Speed
  - Level and Experience tracking
  - Equipment slots and inventory management
  - Proficiencies (weapons, armor, skills, saves, tools)
  - Character resources (HP, spell slots, ability uses)

#### Conditions System
- **Standard D&D 5e Conditions**: All 15 conditions implemented
  - Blinded, Charmed, Deafened, Frightened, Grappled
  - Incapacitated, Invisible, Paralyzed, Petrified, Poisoned
  - Prone, Restrained, Stunned, Unconscious, Exhaustion
- **Mechanical Effects**: Conditions apply appropriate modifiers

#### Effects System
- **ActiveEffect**: Temporary effects with:
  - Duration tracking (rounds, minutes, hours, until rest)
  - Modifiers (damage bonus, resistance, AC bonus, advantage/disadvantage)
  - Concentration tracking
  - Source tracking
- **Effect Manager**: Manages active effects on characters

#### Features System
- **Class Features**: Implemented for all base classes
  - Barbarian: Rage, Unarmored Defense
  - Fighter: Fighting Style, Second Wind
  - Rogue: Sneak Attack, Expertise, Thieves' Cant
  - Monk: Martial Arts, Unarmored Defense
  - Wizard: Spellcasting, Arcane Recovery
  - Cleric: Spellcasting, Divine Domain
  - Ranger: Favored Enemy, Natural Explorer
- **Racial Features**: Implemented for all races
  - Darkvision, resistances, special abilities

#### Combat System
- **Encounter Management**: Full combat encounter tracking
  - Initiative and turn order
  - Combat status tracking
  - Combat log
  - Multiple combatant types (players, monsters, NPCs)
- **Attack System**: Comprehensive attack resolution
  - Attack rolls with modifiers
  - Damage calculation with types
  - Critical hits
  - Advantage/disadvantage
- **Action Economy**: Actions, bonus actions, reactions

#### Event System
- **Comprehensive Event Types**:
  - Attack sequence: BeforeAttackRoll, OnAttackRoll, AfterAttackRoll, BeforeHit, OnHit, AfterHit
  - Damage: BeforeDamageRoll, OnDamageRoll, AfterDamageRoll, BeforeTakeDamage, OnTakeDamage, AfterTakeDamage
  - Saves and checks: BeforeSavingThrow, OnSavingThrow, AfterSavingThrow, BeforeAbilityCheck, OnAbilityCheck, AfterAbilityCheck
  - Turn management: OnTurnStart, OnTurnEnd
  - Status: OnStatusApplied, OnStatusRemoved
  - Rest: OnShortRest, OnLongRest
  - Spells: OnSpellCast, OnSpellDamage
- **Event Bus**: Priority-based event handling with listeners

#### Abilities System
- **Ability Framework**: Generic ability execution system
- **Implemented Abilities**:
  - Rage (Barbarian)
  - Second Wind (Fighter)
  - Sneak Attack (Rogue)
  - Bardic Inspiration (Bard)
  - Lay on Hands (Paladin)
  - Divine Sense (Paladin)
  - Vicious Mockery (Bard)

#### Resources System
- **Resource Tracking**:
  - Hit points (current/max/temp)
  - Spell slots by level
  - Ability uses (rage, second wind, etc.)
  - Hit dice
  - Action economy (action, bonus action, reaction)
- **Rest Mechanics**: Short rest and long rest recovery

#### Equipment System
- **Weapons**: Properties (finesse, versatile, two-handed, etc.)
- **Armor**: AC calculations with different armor types
- **Equipment Slots**: Main hand, off hand, armor, shield

### 2. Discord Bot Architecture Insights

#### Strengths
- Well-structured domain models
- Event-driven architecture already in place
- Clear separation between game mechanics and Discord handlers
- Comprehensive test coverage
- Redis-based persistence with good abstractions

#### Areas for Improvement
- Some mechanics still hardcoded in character methods
- Event system could be more generic
- Features system needs better plugin architecture
- Resource management could be more modular

## Migration Priorities

### Phase 1: Core Systems (Immediate)
1. **Conditions System** ✅ (Already implemented in toolkit)
2. **Effects System** (Current focus)
3. **Event System** (High priority - enables other systems)
4. **Resources System** (Critical for abilities)

### Phase 2: Combat Mechanics
1. **Action Economy**
2. **Attack Resolution**
3. **Damage Calculation**
4. **Initiative and Turn Order**

### Phase 3: Character Features
1. **Feature Framework**
2. **Class Features**
3. **Racial Features**
4. **Proficiency System**

### Phase 4: Equipment and Items
1. **Equipment System**
2. **Weapon Properties**
3. **Armor and AC Calculation**

### Phase 5: Advanced Systems
1. **Spell System**
2. **Ability Framework**
3. **Monster System**

## Implementation Strategy

### For Effects System (Current Module)
Based on the Discord bot's implementation, the effects system should include:

```go
// Key components to implement
1. Effect types matching Discord bot:
   - Duration tracking (rounds, minutes, hours, until rest, permanent)
   - Modifier types (damage bonus, resistance, AC bonus, advantage, etc.)
   - Concentration tracking
   - Source tracking (who/what created the effect)

2. Effect Manager features:
   - Add/remove effects
   - Tick durations
   - Handle concentration
   - Query active effects by type
   - Calculate total modifiers

3. Integration with conditions:
   - Effects can apply conditions
   - Conditions create appropriate effects
   - Proper stacking rules

4. Event integration:
   - Effects can listen to events
   - Effects can modify event results
   - Effects can trigger on specific conditions
```

### For Event System (Next Priority)
The Discord bot's event system provides a good template:

```go
// Event system requirements from Discord bot
1. Event types covering all game mechanics
2. Priority-based listener registration
3. Event context with modifiable data
4. Before/During/After event phases
5. Cancellable events
6. Event results that can be modified by listeners
```

### Integration Approach

1. **Maintain Compatibility**: Design toolkit APIs to be easily adaptable by the Discord bot
2. **Progressive Migration**: Allow Discord bot to use toolkit modules incrementally
3. **Event-Driven Design**: Use events as the primary integration point
4. **Generic Interfaces**: Make toolkit usable by any game system, not just D&D 5e

## Specific Mechanics to Migrate

### From Discord Bot to Toolkit

1. **Rage (Barbarian)**
   - Damage bonus on melee attacks
   - Resistance to physical damage
   - Limited uses per long rest
   - Duration tracking
   - Ends if no attack/damage

2. **Sneak Attack (Rogue)**
   - Extra damage once per turn
   - Requires advantage or ally nearby
   - Finesse or ranged weapons only
   - Scales with level

3. **Second Wind (Fighter)**
   - Healing ability
   - Limited uses per rest
   - Bonus action

4. **Unarmored Defense**
   - Monk: 10 + DEX + WIS
   - Barbarian: 10 + DEX + CON
   - Conditional AC calculation

5. **Fighting Styles** (In Progress)
   - Defense: +1 AC
   - Dueling: +2 damage
   - Great Weapon Fighting: Reroll 1s and 2s (implemented with die reroll tracking)
   - Two-Weapon Fighting: Add ability modifier
   - **Note**: The bot was implementing fighting styles when migration was decided

6. **Spell System** (In Progress)
   - **Magic Missile**: 
     - Auto-hit force damage
     - 3 missiles + 1 per spell level above 1st
     - 1d4+1 damage per missile
     - Can split between targets
     - Event-driven damage application
   - **Vicious Mockery**:
     - Psychic damage cantrip
     - Wisdom save or take damage + disadvantage
     - Scales with character level
   - **Spell Framework**:
     - Spell slot consumption
     - Spell level scaling
     - Event emission (OnSpellCast, OnSpellDamage)
     - Target validation
     - Damage distribution

## Success Metrics

1. **Code Reusability**: Toolkit modules work in multiple contexts
2. **Clean Architecture**: Clear separation of concerns
3. **Performance**: No significant overhead from abstraction
4. **Extensibility**: Easy to add new features/mechanics
5. **Test Coverage**: Comprehensive tests for all modules

## Next Steps

1. **Complete Effects Module** with all features from Discord bot
2. **Design Event System** based on Discord bot's implementation
3. **Create Migration Guide** for Discord bot to use toolkit
4. **Build Example Integrations** showing toolkit usage
5. **Performance Benchmarks** comparing toolkit vs direct implementation

## Notes for Toolkit Development

- The Discord bot has excellent examples of real-world usage
- Many edge cases are already handled in the bot's tests
- The event system is the key to making everything pluggable
- Resources and effects are fundamental to most other systems
- Keep the toolkit generic enough for non-D&D games