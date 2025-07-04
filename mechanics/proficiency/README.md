# Proficiency Module

The proficiency module provides a comprehensive system for handling proficiencies in RPG games, particularly designed for D&D 5e mechanics.

## Features

- **Multiple Proficiency Types**: Weapons, armor, skills, saving throws, tools, and instruments
- **Level-based Bonuses**: Automatic proficiency bonus calculation based on entity level
- **Category Support**: Group proficiencies like "simple-weapons" or "martial-weapons"
- **Flexible Storage**: Interface-based storage with memory implementation provided
- **Thread-safe**: Concurrent access supported through the memory storage

## Usage

### Basic Setup

```go
import (
    "github.com/KirkDiggler/rpg-toolkit/mechanics/proficiency"
)

// Create storage and level provider
storage := proficiency.NewMemoryStorage()
levelProvider := &MyLevelProvider{}

// Create manager
manager := proficiency.NewManager(storage, levelProvider)
```

### Adding Proficiencies

```go
// Add specific weapon proficiency
shortswordProf := proficiency.NewSimpleProficiency(proficiency.SimpleProficiencyConfig{
    Type:     proficiency.ProficiencyTypeWeapon,
    Key:      "shortsword",
    Name:     "Shortsword",
    Category: "", // No category for specific proficiency
})
manager.AddProficiency(entity, shortswordProf)

// Add category proficiency
simpleWeaponsProf := proficiency.NewSimpleProficiency(proficiency.SimpleProficiencyConfig{
    Type:     proficiency.ProficiencyTypeWeapon,
    Key:      "simple-weapons",
    Name:     "Simple Weapons",
    Category: "simple-weapons", // Category matches key for group proficiencies
})
manager.AddProficiency(entity, simpleWeaponsProf)
```

### Checking Proficiencies

```go
// Check specific proficiency
if manager.HasWeaponProficiency(entity, "shortsword") {
    // Entity is proficient with shortswords
}

// Get attack bonus (includes proficiency bonus if proficient)
attackBonus := manager.GetWeaponAttackBonus(entity, "longsword")

// Get skill bonus
skillBonus := manager.GetSkillBonus(entity, "performance")
```

### Proficiency Bonus by Level

The module follows standard D&D 5e proficiency bonus progression:

- Levels 1-4: +2
- Levels 5-8: +3
- Levels 9-12: +4
- Levels 13-16: +5
- Levels 17-20: +6

## Interfaces

### Manager

The main interface for proficiency operations:

```go
type Manager interface {
    // Check proficiency
    HasProficiency(entity core.Entity, profType ProficiencyType, key string) bool
    HasWeaponProficiency(entity core.Entity, weaponKey string) bool
    HasSkillProficiency(entity core.Entity, skillKey string) bool
    
    // Get bonuses
    GetProficiencyBonus(entity core.Entity) int
    GetWeaponAttackBonus(entity core.Entity, weaponKey string) int
    GetSkillBonus(entity core.Entity, skillKey string) int
    GetSaveBonus(entity core.Entity, abilityKey string) int
    
    // Manage proficiencies
    AddProficiency(entity core.Entity, prof Proficiency)
    RemoveProficiency(entity core.Entity, profKey string)
    GetAllProficiencies(entity core.Entity) []Proficiency
}
```

### Storage

Implement this interface for custom storage backends:

```go
type Storage interface {
    GetProficiencies(entityID string) ([]Proficiency, error)
    SaveProficiency(entityID string, prof Proficiency) error
    RemoveProficiency(entityID string, profKey string) error
    HasProficiency(entityID string, profType ProficiencyType, key string) (bool, error)
}
```

### LevelProvider

Implement this to provide entity level information:

```go
type LevelProvider interface {
    GetLevel(entity core.Entity) int
}
```

## Examples

### Rogue with Weapon Proficiency

```go
// Create a rogue entity
rogue := &Character{ID: "rogue-1", Level: 3}

// Add shortsword proficiency
manager.AddProficiency(rogue, proficiency.NewSimpleProficiency(proficiency.SimpleProficiencyConfig{
    Type: proficiency.ProficiencyTypeWeapon,
    Key:  "shortsword",
    Name: "Shortsword",
}))

// Attack with shortsword - gets +2 proficiency bonus
attackBonus := manager.GetWeaponAttackBonus(rogue, "shortsword") // Returns 2
```

### Fighter with Weapon Categories

```go
// Create a fighter entity
fighter := &Character{ID: "fighter-1", Level: 5}

// Add all simple and martial weapon proficiencies
manager.AddProficiency(fighter, proficiency.NewSimpleProficiency(proficiency.SimpleProficiencyConfig{
    Type:     proficiency.ProficiencyTypeWeapon,
    Key:      "simple-weapons",
    Name:     "Simple Weapons",
    Category: "simple-weapons",
}))

manager.AddProficiency(fighter, proficiency.NewSimpleProficiency(proficiency.SimpleProficiencyConfig{
    Type:     proficiency.ProficiencyTypeWeapon,
    Key:      "martial-weapons",
    Name:     "Martial Weapons",
    Category: "martial-weapons",
}))

// Attack with any simple or martial weapon - gets +3 proficiency bonus (level 5)
attackBonus := manager.GetWeaponAttackBonus(fighter, "longsword") // Returns 3
```

## Customization

The module is designed to be extensible. You can:

1. Implement custom `Storage` for database persistence
2. Override category matching logic for your game system
3. Add new proficiency types as needed
4. Customize proficiency bonus calculation

## Thread Safety

The provided `MemoryStorage` implementation is thread-safe using read/write mutexes. Custom storage implementations should also ensure thread safety if concurrent access is expected.