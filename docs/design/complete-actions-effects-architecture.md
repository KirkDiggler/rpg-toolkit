# Complete Actions/Effects Architecture

## Date: 2025-08-16

## Core Insight

Everything in RPGs boils down to two patterns:
1. **Actions**: Things that activate (rage, cast spell, attack, use item)
2. **Effects**: Things that modify via event bus (bless adds 1d4, rage adds damage)

## The Generic Action Pattern

The toolkit provides a generic Action interface that rulebooks implement with their own input types:

```go
// Toolkit provides the interface (in mechanics/actions/action.go)
type Action[T any] interface {
    core.Entity
    GetActivationType() ActivationType  // Action, BonusAction, Reaction, etc.
    CanActivate(ctx context.Context, owner Entity, input T) error
    Activate(ctx context.Context, owner Entity, input T) error
}
```

## Type Constants Architecture

The toolkit provides TYPE definitions, but rulebooks define the CONSTANT values:

### Toolkit Layer (What We Just Implemented in Phase 0)

```go
// github.com/KirkDiggler/rpg-toolkit/core/events/types.go
package events

type EventType string
type ModifierType string  
type ModifierSource string
type Priority string

// github.com/KirkDiggler/rpg-toolkit/core/combat/types.go
package combat

type AttackType string
type WeaponProperty string
type ArmorType string
type ActionType string

// Common examples provided but NOT required
const (
    AttackMeleeWeapon  AttackType = "melee_weapon"
    AttackRangedWeapon AttackType = "ranged_weapon"
)

// github.com/KirkDiggler/rpg-toolkit/core/damage/types.go
package damage

type Type string
type ResistanceType string
type ImmunityType string
```

### Rulebook Layer (Defines Actual Constants)

```go
// github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/constants/events.go
package constants

import "github.com/KirkDiggler/rpg-toolkit/core/events"

// D&D 5e specific event types
const (
    EventAttackRoll     events.EventType = "dnd5e.attack.roll"
    EventDamageRoll     events.EventType = "dnd5e.damage.roll"
    EventSavingThrow    events.EventType = "dnd5e.save.throw"
    EventAbilityCheck   events.EventType = "dnd5e.ability.check"
    EventInitiativeRoll events.EventType = "dnd5e.initiative.roll"
)

// D&D 5e specific modifier types
const (
    ModifierProficiency events.ModifierType = "proficiency"
    ModifierAdvantage   events.ModifierType = "advantage"
    ModifierDisadvantage events.ModifierType = "disadvantage"
    ModifierBless       events.ModifierType = "bless"
    ModifierGuidance    events.ModifierType = "guidance"
)

// github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/constants/damage.go
package constants

import "github.com/KirkDiggler/rpg-toolkit/core/damage"

// D&D 5e damage types
const (
    DamageBludgeoning damage.Type = "bludgeoning"
    DamagePiercing    damage.Type = "piercing"
    DamageSlashing    damage.Type = "slashing"
    DamageFire        damage.Type = "fire"
    DamageCold        damage.Type = "cold"
    DamageAcid        damage.Type = "acid"
    DamagePoison      damage.Type = "poison"
    DamageNecrotic    damage.Type = "necrotic"
    DamageRadiant     damage.Type = "radiant"
    DamagePsychic     damage.Type = "psychic"
    DamageForce       damage.Type = "force"
    DamageThunder     damage.Type = "thunder"
    DamageLightning   damage.Type = "lightning"
)
```

## Complete Example: Rage Feature

### 1. Rage Action Implementation (Rulebook)

```go
// github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/features/rage.go
package features

import (
    "context"
    "errors"
    "github.com/KirkDiggler/rpg-toolkit/core"
    "github.com/KirkDiggler/rpg-toolkit/core/damage"
    "github.com/KirkDiggler/rpg-toolkit/events"
    "github.com/KirkDiggler/rpg-toolkit/mechanics/actions"
    "github.com/KirkDiggler/rpg-toolkit/mechanics/resources"
    "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/constants"
)

// RageInput is what the rulebook decides rage needs
type RageInput struct {
    // Empty for rage, but the pattern supports any input
}

// Rage implements Action[RageInput]
type Rage struct {
    id     string
    uses   resources.Resource
}

// Implement Action[RageInput] interface
func (r *Rage) GetID() string { return r.id }
func (r *Rage) GetType() string { return "feature" }
func (r *Rage) GetActivationType() actions.ActivationType { 
    return actions.ActivationBonusAction 
}

func (r *Rage) CanActivate(ctx context.Context, owner core.Entity, input RageInput) error {
    if r.uses.Current() <= 0 {
        return errors.New("no rage uses remaining")
    }
    
    // Check if already raging via event bus
    bus := events.FromContext(ctx)
    query := bus.Query(events.QueryInput{
        EventType: constants.EventStatusCheck,
        Target: owner,
        Data: map[string]any{"status": "raging"},
    })
    if query.Result.(bool) {
        return errors.New("already raging")
    }
    
    return nil
}

func (r *Rage) Activate(ctx context.Context, owner core.Entity, input RageInput) error {
    // Consume resource
    r.uses.Consume(1)
    
    // Register rage effects via event bus
    bus := events.FromContext(ctx)
    
    // Add damage bonus modifier
    bus.Publish(events.Event{
        Type: constants.EventModifierAdd,
        Source: r,
        Target: owner,
        Data: ModifierData{
            Type: constants.ModifierRageDamage,
            Source: constants.SourceRage,
            Value: 2, // +2 damage at low levels
            Duration: "10_minutes",
            AppliesTo: []events.EventType{
                constants.EventMeleeAttackDamage,
            },
        },
    })
    
    // Add damage resistance
    bus.Publish(events.Event{
        Type: constants.EventResistanceAdd,
        Source: r,
        Target: owner,
        Data: ResistanceData{
            Types: []damage.Type{
                constants.DamageBludgeoning,
                constants.DamagePiercing,
                constants.DamageSlashing,
            },
            Source: constants.SourceRage,
            Duration: "10_minutes",
        },
    })
    
    return nil
}
```

### 2. Character Loading (API Layer)

```go
// github.com/KirkDiggler/rpg-api/internal/loaders/character_loader.go
package loaders

import (
    "github.com/KirkDiggler/rpg-toolkit/mechanics/actions"
    "github.com/KirkDiggler/rpg-api/internal/repositories"
    dnd5e "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e"
)

type CharacterLoader struct {
    repo     repositories.CharacterRepository
    rulebook dnd5e.Rulebook  // Knows it's D&D 5e
}

func (l *CharacterLoader) Load(ctx context.Context, id string) (*Character, error) {
    // Get data from repository
    data, err := l.repo.Get(ctx, id)
    if err != nil {
        return nil, err
    }
    
    // Create character with compiled values
    char := &Character{
        ID: data.ID,
        Name: data.Name,
        // All values are pre-compiled in the database
        STR: data.CompiledSTR,  // e.g., 18 for a raging barbarian
        DEX: data.CompiledDEX,
        // ... etc
    }
    
    // Load features as Actions
    for _, featureData := range data.Features {
        switch featureData.Type {
        case "rage":
            // We know it's D&D 5e, so we can create Rage
            rage := l.rulebook.CreateRage(featureData)
            char.AddAction(rage)
            
        case "second_wind":
            secondWind := l.rulebook.CreateSecondWind(featureData)
            char.AddAction(secondWind)
            
        // ... other D&D 5e features
        }
    }
    
    // Load spells as Actions
    for _, spellData := range data.KnownSpells {
        spell := l.rulebook.CreateSpell(spellData)
        char.AddAction(spell)
    }
    
    return char, nil
}
```

### 3. Character with Generic Actions

```go
// github.com/KirkDiggler/rpg-api/internal/entities/character.go
package entities

type Character struct {
    ID      string
    Name    string
    
    // Compiled values from database
    STR     int
    DEX     int
    CON     int
    INT     int
    WIS     int
    CHA     int
    
    // Actions can be features, spells, items, etc.
    // Character doesn't know the types, just stores them
    actions map[string]any // Actually Action[T] but stored as any
}

// ActivateAction is the generic activation method
func (c *Character) ActivateAction(ctx context.Context, ref string, input any) error {
    action, exists := c.actions[ref]
    if !exists {
        return ErrActionNotFound
    }
    
    // The action knows its own input type and handles it
    // This is where the type erasure happens - we trust the action
    return activateGeneric(ctx, c, action, input)
}

// Helper to handle the generic dispatch
func activateGeneric(ctx context.Context, owner Entity, action any, input any) error {
    // Each action type handles its own input
    // This is internal to the implementation
    switch a := action.(type) {
    case *features.Rage:
        // Rage expects RageInput (empty struct)
        return a.Activate(ctx, owner, input.(features.RageInput))
        
    case *spells.Fireball:
        // Fireball expects targeting info
        return a.Activate(ctx, owner, input.(spells.TargetingInput))
        
    // ... other action types
    }
    
    return errors.New("unknown action type")
}
```

### 4. Complete Flow Example

```go
// In the game server / Discord bot handler
func HandleRageCommand(ctx context.Context, playerID string) error {
    // 1. Load character (with all pre-compiled values and actions)
    char, err := loader.Load(ctx, playerID)
    if err != nil {
        return err
    }
    
    // 2. Activate the rage action
    // The character has rage because the loader saw it in the database
    err = char.ActivateAction(ctx, "barbarian:rage", features.RageInput{})
    if err != nil {
        return err  // "no uses remaining" or "already raging"
    }
    
    // 3. Rage is now active via event bus modifiers
    // Next attack will automatically get +2 damage
    // Character has resistance to physical damage
    
    return nil
}

func HandleAttackCommand(ctx context.Context, playerID string, targetID string) error {
    char, _ := loader.Load(ctx, playerID)
    target, _ := loader.Load(ctx, targetID)
    
    // Create attack event
    bus := events.FromContext(ctx)
    
    // This will gather all modifiers including rage damage if active
    result := bus.Publish(events.Event{
        Type: constants.EventMeleeAttackDamage,
        Source: char,
        Target: target,
        Data: AttackData{
            Weapon: "greataxe",
            BaseDamage: "1d12",
        },
    })
    
    // Modifiers automatically applied:
    // - Rage damage (+2) if raging
    // - Bless (+1d4) if blessed
    // - Strength modifier
    // - Proficiency bonus
    // etc.
    
    return nil
}
```

## Key Architecture Points

### 1. Toolkit Provides Types, Rulebook Provides Values
- Toolkit: `type EventType string`
- Rulebook: `const EventAttackRoll EventType = "dnd5e.attack.roll"`

### 2. Actions are Generic
- Interface: `Action[T any]`
- Rage: `Action[RageInput]`
- Fireball: `Action[TargetingInput]`
- Each action defines its own input

### 3. Character Stores Compiled Values
- No calculations at runtime
- Database has STR: 18, not base: 15 + racial: 2 + asi: 1
- Proficiency bonus is just a number: 3

### 4. Loader Knows the Rulebook
- API layer knows it's implementing D&D 5e
- Can create specific types (Rage, Fireball)
- But Character just stores them generically

### 5. Everything Else is Events
- Modifiers applied via event bus
- Conditions checked via event queries
- Damage calculated via event aggregation

## What This Achieves

1. **Type Safety**: Each action has typed input
2. **No Magic Strings**: Everything uses typed constants
3. **No Reflection**: Generic dispatch is compile-time safe
4. **Clean Separation**: Toolkit doesn't know about specific features
5. **Data Driven**: Everything loaded from database
6. **Event Driven**: All runtime behavior through events

## Implementation Phases

### Phase 0: Typed Constants ✅ (Just Completed)
- Added type definitions to toolkit packages
- Created damage and combat packages
- Foundation for everything else

### Phase 1: Action Interface
- Generic Action[T] interface
- Basic activation framework
- Input type patterns

### Phase 2: Rage Implementation
- First concrete action
- Resource consumption
- Event-based modifiers

### Phase 3: Bless Implementation
- Spell as action
- Concentration via events
- Dice modifiers

### Phase 4: Loader Pattern
- Repository → Loader → Orchestrator
- Database to domain objects
- Type-specific creation

## Summary

The Actions/Effects architecture gives us:
- **Actions**: Generic interface with typed inputs
- **Effects**: Event-based modifiers
- **Constants**: Toolkit types, rulebook values
- **Loading**: API knows rulebook, creates typed actions
- **Runtime**: Everything else via events

This is the complete picture of what we designed!