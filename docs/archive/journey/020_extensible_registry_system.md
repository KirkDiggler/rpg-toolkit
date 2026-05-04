# Journey 020: Extensible Registry System for D&D 5e Content

## The Problem

As we implemented typed constants for D&D 5e (ClassFighter, RaceHuman, etc.), we gained excellent type safety. However, D&D is a living game - WotC has added:
- New classes (Artificer)
- New subclasses (Echo Knight, Chronurgy Wizard)
- New races (Fairy, Harengon)
- New spells with complex behaviors
- Setting-specific content (Eberron, Ravnica)

Our typed constants approach doesn't allow for this extensibility. We need a system that:
1. Maintains type safety for core content
2. Allows modules to add new content seamlessly
3. Handles both data AND behavior (spells need implementation, not just description)
4. Feels natural to use - extended content should work just like core content

## The Journey

### Starting Point: Pure Constants
```go
// What we have now
type Class string
const (
    ClassFighter Class = "fighter"
    ClassWizard  Class = "wizard"
    // But no ClassArtificer!
)
```

### First Realization: We Need Data + Behavior
Kirk: "dnd5e is mostly rules but knowing what a given spell does is needed to be coded. so this is not just data we want to inject but behaviour too"

This is key - a spell isn't just:
```go
type SpellData struct {
    Name        string
    Level       int
    Description string
}
```

It needs behavior:
```go
type Spell interface {
    Cast(caster Character, targets []Target) (*SpellResult, error)
}
```

### The Module Vision
Kirk: "would like it to be something that say a module for artificer could bring in new feats or other things"

A module isn't just one thing - it's a complete package:
- Classes
- Subclasses  
- Spells (with implementations)
- Feats
- Items
- Rules modifications

### The Registry Pattern Emerges

We need a registry that can:
1. Hold core content with type safety
2. Accept extended content at runtime
3. Make both feel the same to use

```go
// Core content - compile time type safe
const ClassFighter Class = "fighter"

// Extended content - runtime registered but validated
var ClassArtificer = registry.RegisterClass("artificer", &ClassData{...})

// Usage feels the same!
fighter := registry.GetClass(ClassFighter) // Type safe
artificer := registry.GetClass(ClassArtificer) // Also "type safe" after registration
```

## The Design

### Hybrid Approach: Constants + Registry

```go
// core/constants.go - Type safe core content
package constants

type Class string
const (
    ClassBarbarian Class = "barbarian"
    ClassBard      Class = "bard"
    ClassCleric    Class = "cleric"
    // ... all PHB classes
)

// registry/registry.go - Extensible registry
package registry

type Registry struct {
    // Core content is pre-registered
    classes map[string]*ClassEntry
}

// During init, register all core content
func init() {
    defaultRegistry.RegisterClass(string(constants.ClassFighter), &ClassEntry{
        Data: fighterData,
        OnLevelUp: fighterLevelUp,
    })
}

// Extended content registers itself
func RegisterClass(id string, entry *ClassEntry) Class {
    defaultRegistry.classes[id] = entry
    return Class(id) // Return a typed ID
}
```

### Module System

```go
// modules/artificer/module.go
package artificer

import "registry"

// Module registers all artificer content
func init() {
    // This creates a typed constant at runtime!
    ClassArtificer = registry.RegisterClass("artificer", &ClassEntry{
        Data: artificerData,
        OnLevelUp: artificerLevelUp,
    })
    
    // Register spells with behavior
    registry.RegisterSpell("arcane_weapon", &SpellEntry{
        Data: arcaneWeaponData,
        Cast: castArcaneWeapon,
    })
}

// Exported "constant" that modules can use
var ClassArtificer Class

// Spell implementation
func castArcaneWeapon(caster Character, targets []Target, slot int) (*SpellResult, error) {
    // Actual game mechanics here
    weapon := targets[0].(*Weapon)
    return &SpellResult{
        Effects: []Effect{{
            Type: "enhancement",
            Target: weapon,
            Bonus: 1,
            Duration: Duration{1, "hour"},
        }},
    }, nil
}
```

### Usage Feels Natural

```go
// Using core content (compile-time type safe)
fighter := NewCharacter(constants.ClassFighter)

// Using extended content (runtime validated but feels the same)
artificer := NewCharacter(artificer.ClassArtificer)

// Both work identically
fighter.LevelUp()   // Uses fighter behavior
artificer.LevelUp() // Uses artificer behavior

// Spells work the same way
spell := registry.GetSpell("fireball")     // Core spell
spell2 := registry.GetSpell("arcane_weapon") // Artificer spell
```

## Key Insights

1. **Type Safety Gradient**: Core content is 100% type safe at compile time. Extended content is validated at registration time and then behaves as if it were type safe.

2. **Behavior is First Class**: Modules don't just add data, they add game mechanics. Each spell knows how to cast itself.

3. **Modules are Packages**: An artificer module brings everything - the class, subclasses, spells, infusions, magic items. It's a complete game expansion.

4. **Registration Time Validation**: When a module registers content, we validate it immediately. This catches errors at startup, not during gameplay.

## Implementation Path

1. **Phase 1**: Keep existing constants, add registry alongside
   - No breaking changes
   - Core content works both ways
   - Start experimenting with modules

2. **Phase 2**: Migrate spell system to registry
   - Spells need behavior anyway
   - Good proof of concept
   - High value - enables spell modules

3. **Phase 3**: Enable class/race modules
   - Artificer as first test case
   - Validate the module packaging approach
   - Ensure it "feels" right

4. **Phase 4**: Full migration
   - All content goes through registry
   - Constants become registry lookups
   - But usage stays the same

## Open Questions

1. How do we handle dependencies? (Artificer needs certain core spells)
2. Should modules be able to modify core content? (variant rules)
3. How do we handle conflicts? (two modules define same content)
4. Persistence - how do we save which modules are active?

## The Breakthrough: Keep Constants, Add Registry

The key realization: we don't need to choose between type-safe constants and extensible registry - we can have both! Our constants are just typed strings, and the registry uses strings as keys. They're already compatible.

```go
// constants stay exactly as they are
const ClassFighter Class = "fighter"

// Registry enhances them with behavior
registry.RegisterClass(string(ClassFighter), &ClassEntry{
    Data: fighterData,
    OnLevelUp: fighterLevelUp,
})

// Modules add new content to the same registry
var ClassArtificer = registry.RegisterClass("artificer", &ClassEntry{
    Data: artificerData,
    OnLevelUp: artificerLevelUp,
})

// Both work identically!
registry.GetClass(string(ClassFighter))   // Core class
registry.GetClass(string(ClassArtificer)) // Extended class
```

No breaking changes. No migration. Just pure enhancement.

## Module Package Architecture

Modules live as separate Go packages that can come from anywhere:

```
# Core toolkit (with constants)
github.com/KirkDiggler/rpg-toolkit

# Official extended modules  
github.com/KirkDiggler/rpg-toolkit-artificer
github.com/KirkDiggler/rpg-toolkit-xanathar

# Community modules
github.com/cool-gm/rpg-toolkit-spelljammer
github.com/homebrew/rpg-toolkit-custom-world
```

Import what you need:
```go
import (
    _ "github.com/KirkDiggler/rpg-toolkit-artificer" // Auto-registers
)
```

## Conclusion

The journey from typed constants to an extensible registry shows the evolution of understanding. We started wanting type safety (good!) but realized D&D's living nature requires extensibility (also good!). 

The breakthrough: keep the type-safe constants AND add the registry. No breaking changes, just pure addition. Core content uses constants + registry, extended content uses just registry, and they work identically.

The key insight: make extended content *feel* like core content once registered. The artificer class should be just as "real" as the fighter class after its module loads.