# ADR-0014: D&D 5e Module and Registry System

Date: 2025-01-29

## Status

Proposed

## Context

The D&D 5e ruleset implementation currently uses typed constants for game content (classes, races, spells, etc.). This provides excellent type safety but lacks extensibility. D&D is a living game with regular expansions:

- New classes (Artificer)
- New subclasses (Echo Knight, Aberrant Mind Sorcerer)  
- New races (Fairy, Harengon, Autognome)
- New spells with complex mechanical implementations
- Setting-specific content (Eberron, Ravnica, Spelljammer)
- Third-party content (Critical Role, Kobold Press)
- Homebrew content

Current limitations:
1. Cannot add new content without modifying core toolkit
2. No way to package related content together (e.g., Artificer class + infusions + spells)
3. No mechanism for content to include behavior (spell implementations)
4. Community content requires forking

## Decision

Implement a **registry system with modular content packages** that:

1. **Preserves existing typed constants** - no breaking changes
2. **Adds a behavior registry** alongside constants
3. **Enables modular content packages** as separate Go modules
4. **Supports both data and behavior** in modules

### Architecture

```
rpg-toolkit/
├── rulebooks/
│   └── dnd5e/
│       ├── constants/        # Existing typed constants (unchanged)
│       ├── registry/         # New registry system
│       └── modules/          # Module interface definitions

# Separate module packages
rpg-toolkit-artificer/        # Official Artificer module
rpg-toolkit-xanathar/         # Xanathar's Guide content
rpg-toolkit-homebrew-x/       # Community modules
```

### Core Design

```go
// registry/registry.go
type Registry struct {
    classes map[string]*ClassEntry
    spells  map[string]*SpellEntry
    races   map[string]*RaceEntry
    feats   map[string]*FeatEntry
}

type ClassEntry struct {
    Data      *ClassData
    OnLevelUp func(char Character, level int) error
    Resources func(char Character) map[string]Resource
}

type SpellEntry struct {
    Data     *SpellData
    Cast     func(caster Character, targets []Target, slot int) (*SpellResult, error)
    Validate func(caster Character, targets []Target) error
}

// Initialize with core content
func init() {
    // Register all constants with behavior
    RegisterClass(string(constants.ClassFighter), &ClassEntry{
        Data:      getFighterData(),
        OnLevelUp: fighterLevelUp,
    })
    // ... etc for all core content
}
```

### Module Interface

```go
// modules/module.go
type Module interface {
    ID() string
    Name() string
    Version() string
    MinToolkitVersion() string
    
    Register(r *Registry) error
}
```

### Example Module

```go
// github.com/KirkDiggler/rpg-toolkit-artificer
package artificer

import "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/registry"

// Exported "constants" for type safety
var (
    ClassArtificer = registry.ClassID("artificer")
    SpellArcaneWeapon = registry.SpellID("arcane_weapon")
)

func init() {
    registry.RegisterModule(&ArtificerModule{})
}

type ArtificerModule struct{}

func (m *ArtificerModule) Register(r *registry.Registry) error {
    // Register class with behavior
    r.RegisterClass(string(ClassArtificer), &registry.ClassEntry{
        Data:      getArtificerData(),
        OnLevelUp: artificerLevelUp,
        Resources: artificerResources,
    })
    
    // Register spells with implementations
    r.RegisterSpell(string(SpellArcaneWeapon), &registry.SpellEntry{
        Data: getArcaneWeaponData(),
        Cast: castArcaneWeapon,
    })
    
    return nil
}

// Spell implementation
func castArcaneWeapon(caster Character, targets []Target, slot int) (*SpellResult, error) {
    // Actual game mechanics
    weapon := targets[0].(*Weapon)
    return &SpellResult{
        Effects: []Effect{{
            Type:     "enhancement",
            Target:   weapon,
            Value:    1,
            Duration: Duration{Amount: 1, Unit: "hour"},
        }},
    }, nil
}
```

### Usage

```go
import (
    "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e"
    "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/constants"
    _ "github.com/KirkDiggler/rpg-toolkit-artificer" // Auto-registers
)

// Core content (unchanged)
fighter := NewCharacter(constants.ClassFighter)

// Extended content (feels the same)
artificer := NewCharacter(artificer.ClassArtificer)

// Both work identically through registry
fighter.LevelUp()   // Uses fighter behavior from registry
artificer.LevelUp() // Uses artificer behavior from registry
```

## Consequences

### Positive

1. **No breaking changes** - Existing code continues to work
2. **Type safety preserved** - Core content keeps compile-time checking
3. **Extensibility** - New content can be added without modifying toolkit
4. **Behavior included** - Spells and abilities have implementations
5. **Modular** - Pick only the content you need
6. **Community friendly** - Easy to create and share modules
7. **Version independent** - Modules can update separately from toolkit

### Negative

1. **Runtime registration** - Extended content validated at startup, not compile time
2. **Dependency management** - Need to ensure module compatibility
3. **Potential conflicts** - Two modules could register same content
4. **Slight complexity** - Two ways to reference content (constant vs registry)

### Neutral

1. **Registry overhead** - Small runtime cost for flexibility
2. **Module discovery** - Need documentation of available modules
3. **Testing complexity** - Modules need integration tests

## Implementation Plan

### Phase 1: Registry Infrastructure (No breaking changes)
1. Create registry package
2. Register all constants with basic behavior
3. Ensure existing code works unchanged

### Phase 2: Spell Behavior System
1. Implement spell registry with behavior
2. Convert core spells to use registry
3. Create example spell module

### Phase 3: Module System
1. Define module interface
2. Create artificer module as proof of concept
3. Document module creation process

### Phase 4: Community Enablement
1. Create module template/generator
2. Set up module registry/discovery
3. Create contribution guidelines

## Alternatives Considered

1. **Code generation** - Generate constants from data files
   - Rejected: Less flexible, requires build step, can't add behavior

2. **Pure registry** - Remove constants entirely
   - Rejected: Breaking change, loses type safety

3. **Fork per variant** - Maintain separate branches
   - Rejected: Maintenance nightmare, fragmentation

4. **Configuration files** - JSON/YAML content definitions
   - Rejected: Can't include behavior, not Go idiomatic

## References

- Journey 020: Extensible Registry System
- Issue #144: Feat selection system (would use registry)
- Issue #146: Spell system (needs behavior)
- D&D 5e SRD: Core content definition
- Go plugin system (considered but rejected for portability)