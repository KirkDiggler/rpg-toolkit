# Identifier Package

The identifier package provides a type-safe, extensible pattern for identifying game mechanics like features, proficiencies, skills, and conditions. It enables external modules to add new identifiers while maintaining type safety for known identifiers.

## Purpose

This package solves the fundamental tension between:
- **Type Safety**: Using hardcoded constants that prevent typos and enable compile-time checking
- **Extensibility**: Allowing external modules (homebrew, third-party content) to add new mechanics

## Core Concepts

### Identifier Structure

An identifier consists of three parts:
- **Module**: Which module defined this ID (`"core"`, `"artificer"`, `"homebrew"`, etc.)
- **Type**: What category of mechanic (`"feature"`, `"proficiency"`, `"skill"`, `"condition"`)
- **Value**: The unique identifier within that module/type namespace (`"rage"`, `"sneak_attack"`)

### String Format

Identifiers serialize to a compact string format: `module:type:value`

Examples:
- `"core:feature:rage"` - The Rage feature from core D&D 5e
- `"artificer:feature:infusions"` - Infusions from an Artificer expansion
- `"core:proficiency:simple_weapons"` - Simple weapons proficiency
- `"homebrew:condition:blessed_by_thor"` - Custom condition from homebrew content

## Usage

### Creating Identifiers

```go
// For compile-time constants (core modules)
var Rage = identifier.MustNew("rage", "core", "feature")
var SneakAttack = identifier.MustNew("sneak_attack", "core", "feature")

// For runtime creation (with validation)
id, err := identifier.New("infusions", "artificer", "feature")
if err != nil {
    return err
}
```

### Source Tracking

Track where an identifier came from (which race, class, or background granted it):

```go
// Character gains Darkvision from being an Elf
feature := identifier.NewWithSource(
    identifier.MustNew("darkvision", "core", "feature"),
    "race:elf",
)

// Character gains Rage from Barbarian class
feature := identifier.NewWithSource(
    Rage,
    "class:barbarian",
)
```

### In Character Data

```go
type CharacterData struct {
    // Features with their sources tracked
    Features []identifier.WithSource `json:"features"`
    
    // Example data:
    // [
    //   {"id": "core:feature:darkvision", "source": "race:elf"},
    //   {"id": "core:feature:rage", "source": "class:barbarian"},
    //   {"id": "core:feature:second_wind", "source": "class:fighter"}
    // ]
}
```

### JSON Serialization

The package supports clean JSON serialization:

```json
{
  "features": [
    {
      "id": "core:feature:rage",
      "source": "class:barbarian"
    },
    {
      "id": "core:feature:darkvision", 
      "source": "race:elf"
    }
  ]
}
```

## Extensibility Pattern

External modules can create their own identifiers without modifying core:

```go
// In an artificer module
package artificer

import "github.com/KirkDiggler/rpg-toolkit/mechanics/identifier"

var (
    // Define artificer-specific features
    Infusions = identifier.MustNew("infusions", "artificer", "feature")
    ArcaneArmament = identifier.MustNew("arcane_armament", "artificer", "feature")
    
    // Define artificer-specific proficiencies
    TinkerTools = identifier.MustNew("tinker_tools", "artificer", "proficiency")
)
```

## Integration with Features

The identifier system works with the feature loading pattern:

```go
func LoadFeatureFromIdentifier(id identifier.WithSource, owner core.Entity) (features.Feature, error) {
    switch id.ID.String() {
    case "core:feature:rage":
        return CreateRageFeature(owner), nil
    case "core:feature:sneak_attack":
        return CreateSneakAttackFeature(owner), nil
    default:
        // Passive feature - no active behavior needed
        return nil, features.ErrPassiveFeature
    }
}
```

## Design Principles

1. **No Central Registry**: Identifiers don't need to be registered globally
2. **Type Safety Where Possible**: Core modules use compile-time constants
3. **Extensibility**: External modules can add identifiers freely
4. **Source Tracking**: Always know where a feature/proficiency came from
5. **Clean Serialization**: Compact, readable JSON format

## Error Handling

The package validates identifiers and returns clear errors:

```go
// Returns error for invalid identifiers
id, err := identifier.New("", "core", "feature")
// Error: identifier value cannot be empty

id, err := identifier.New("rage", "", "feature")  
// Error: identifier module cannot be empty
```

## Future Considerations

- Registry for known identifiers (optional, for validation)
- Namespace collision detection
- Version compatibility tracking
- Dependency resolution between identifiers