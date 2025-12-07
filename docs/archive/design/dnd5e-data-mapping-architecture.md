# D&D 5e Data Mapping Architecture

## Overview

This document describes how the rpg-toolkit provides typed D&D 5e data structures and mapping tools that game servers can use to convert external data sources (APIs, databases) into strongly-typed domain objects.

## Design Principles

1. **No Registries** - Use constructors and behavior patterns instead of global registries
2. **Data-Driven Effects** - Spells and items expose effects as data that game servers interpret
3. **Interface-Based** - Game servers work with interfaces, not concrete types
4. **LoadData Pattern** - Single entry point for creating objects from external data
5. **Domain-Specific Packages** - Organize constants by domain (skills.Athletics, races.Dwarf)

## Architecture

### Layer Separation

```
External API → External Client → Toolkit Types → Game Server → Game Engine
     ↓              ↓                  ↓             ↓            ↓
  (unsafe)     (converts)         (interfaces)   (business)  (interprets)
```

### Key Components

1. **Toolkit** (rpg-toolkit/rulebooks/dnd5e)
   - Provides interfaces and data structures
   - Defines effect types
   - Implements LoadData functions
   - No game logic, just structure

2. **External Client** (game server's adapter)
   - Fetches from external sources
   - Converts to toolkit SpellData/ItemData
   - Enriches with known effects
   - Returns toolkit interfaces

3. **Game Server** (orchestration layer)
   - Uses toolkit interfaces
   - Doesn't know about specific spells/items
   - Passes effects to game engine

4. **Game Engine** (interpretation layer)
   - Interprets effect types
   - Implements game mechanics
   - Handles world state changes

## Package Structure

### Domain-Specific Constants

```
rulebooks/dnd5e/
├── abilities/       # STR, DEX, CON, INT, WIS, CHA
├── skills/          # Athletics, Acrobatics, etc.
├── races/           # Dwarf, Elf, Human, etc.
├── classes/         # Fighter, Wizard, Rogue, etc.
├── weapons/         # Categories, properties
├── armors/          # Light, Medium, Heavy, Shield
├── languages/       # Common, Elvish, Dwarvish, etc.
├── backgrounds/     # Soldier, Criminal, Noble, etc.
├── sizes/           # Tiny, Small, Medium, Large, etc.
├── spells/          # Spell interface and LoadData
├── items/           # Item interface and LoadData
└── mappings/        # Generic mapper utilities
```

### Usage Examples

```go
// Using domain-specific constants
if character.Race == races.Dwarf {
    // Apply dwarven resilience
}

if character.HasSkill(skills.Athletics) {
    // Add proficiency bonus
}

// Creating race with variadic options
raceData := races.New(races.Dwarf,
    races.WithSpeed(25),
    races.WithAbilityBonuses(map[abilities.Ability]int{
        abilities.CON: 2,
    }),
)
```

## Spell System Design

### Spell Interface

```go
type Spell interface {
    // Identity
    GetID() string
    GetName() string
    GetLevel() int
    
    // Execution
    Cast(ctx *CastContext) (*CastResult, error)
    
    // Effects for game server interpretation
    GetEffects() []Effect
}
```

### Effect Types

Effects are data structures that game servers can interpret:

- **DamageEffect** - Dice, damage type, saving throw
- **HealingEffect** - Dice, target restrictions
- **LightEffect** - Radius, duration, color
- **SummonEffect** - Entity type, stats, abilities
- **ConditionEffect** - Condition to apply, duration
- **ModifierEffect** - Stat modifications
- **UtilityEffect** - Generic effects with parameters

### LoadData Pattern

```go
// The ONLY way game servers create spells
spell, err := spells.LoadData(spells.SpellData{
    ID:     "fireball",
    Name:   "Fireball",
    Level:  3,
    School: "evocation",
    Damage: &spells.DamageData{
        Dice: "8d6",
        Type: "fire",
        SaveType: "dexterity",
        SaveForHalf: true,
    },
})
```

## Item System Design

### Item Interface

```go
type Item interface {
    GetID() string
    GetName() string
    GetType() ItemType
    GetRarity() Rarity
    
    // For equipment
    Equip(character Character) error
    Unequip(character Character) error
    
    // Effects when equipped/used
    GetEffects() []Effect
    GetModifiers() []Modifier
}
```

### LoadData for Items

```go
item, err := items.LoadData(items.ItemData{
    ID:     "longsword",
    Name:   "Longsword",
    Type:   "weapon",
    Damage: &items.DamageData{
        Dice: "1d8",
        Type: "slashing",
    },
})
```

## External Client Pattern

### Interface

```go
type Client interface {
    // Returns toolkit types directly
    GetRace(ctx context.Context, raceID string) (*races.Data, error)
    GetClass(ctx context.Context, classID string) (*classes.Data, error)
    GetSpell(ctx context.Context, spellID string) (spells.Spell, error)
    GetItem(ctx context.Context, itemID string) (items.Item, error)
}
```

### Implementation

The external client:
1. Fetches from external API
2. Converts to toolkit data structures
3. Enriches with known effects (for spells like Light, Mage Hand)
4. Returns toolkit interfaces

```go
func (c *client) GetSpell(ctx context.Context, spellID string) (spells.Spell, error) {
    // Fetch from API
    apiSpell, err := c.api.GetSpell(ctx, spellID)
    if err != nil {
        return nil, err
    }
    
    // Convert to spell data
    data := c.convertToSpellData(apiSpell)
    
    // Enrich with known effects
    data = c.enrichWithKnownEffects(data)
    
    // Use LoadData to create spell
    return spells.LoadData(data)
}
```

## Game Server Integration

### Orchestrator Layer

```go
func (o *Orchestrator) CastSpell(ctx context.Context, input *CastSpellInput) (*CastSpellOutput, error) {
    // Get spell interface from external client
    spell, err := o.external.GetSpell(ctx, input.SpellID)
    if err != nil {
        return nil, err
    }
    
    // Execute through game engine
    err = o.engine.ExecuteSpell(spell, input.Caster, input.Targets)
    
    return &CastSpellOutput{
        SpellCast: spell.GetName(),
        Effects:   spell.GetEffects(),
    }, nil
}
```

### Game Engine Interpretation

```go
func (e *SpellExecutor) ExecuteSpell(spell spells.Spell, caster Entity, targets []Entity) error {
    // Get effects from spell
    effects := spell.GetEffects()
    
    for _, effect := range effects {
        switch effect.Type() {
        case spells.EffectTypeDamage:
            e.handleDamage(effect.(spells.DamageEffect), caster, targets)
            
        case spells.EffectTypeLight:
            e.handleLight(effect.(spells.LightEffect), caster)
            
        case spells.EffectTypeSummon:
            e.handleSummon(effect.(spells.SummonEffect), caster)
            
        // ... other effect types
        }
    }
    
    return nil
}
```

## Benefits

1. **Type Safety** - Strong typing for known values (races, classes, skills)
2. **Flexibility** - Unknown spells/items work through data
3. **Clean Boundaries** - Clear separation between toolkit and game server
4. **No Global State** - No registries, just data and behavior
5. **Extensible** - New effect types can be added
6. **Game-Agnostic** - Toolkit doesn't know about game implementation

## Migration Path

For existing systems:

1. **Phase 1**: Implement domain-specific constant packages
2. **Phase 2**: Create LoadData functions for spells/items
3. **Phase 3**: Update external client to return toolkit types
4. **Phase 4**: Refactor orchestrators to use interfaces
5. **Phase 5**: Implement effect interpretation in game engine

## Future Considerations

- **Custom Effects** - Allow game servers to register custom effect types
- **Effect Validation** - Validate effect combinations
- **Effect Composition** - Complex effects built from simple ones
- **Performance** - Cache loaded spells/items
- **Versioning** - Handle different D&D 5e versions/sources

## Related Documents

- [Journey: Data-Driven Architecture Discovery](../journey/024-data-driven-runtime-architecture.md)
- [ADR: Effect Composition Pattern](../adr/0024-effect-composition-pattern.md) (future)
- [Example: Spell Implementation](../examples/spell-implementation.md) (future)