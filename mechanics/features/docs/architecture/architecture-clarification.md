# Architecture Clarification: Where Things Actually Live

## The Key Insight

**The Ref constant should live with the implementation, not in a separate "refs" package.**

## Package Structure

```
rulebooks/
└── dnd5e/
    ├── classes/
    │   └── barbarian/
    │       ├── barbarian.go      // Class definition
    │       └── features.go        // Rage lives here!
    ├── conditions/
    │   └── conditions.go          // Poisoned lives here!
    └── spells/
        └── fireball.go            // Fireball lives here!
```

## Example: Rage

```go
// rulebooks/dnd5e/classes/barbarian/features.go
package barbarian

import (
    "github.com/KirkDiggler/rpg-toolkit/core"
    "github.com/KirkDiggler/rpg-toolkit/mechanics/features"
)

// RageRef is the identifier for the Rage feature
var RageRef = core.MustNewRef(core.RefInput{
    Module: "dnd5e",
    Type:   "feature",
    Value:  "rage",
})

// NewRage creates a new Rage feature instance
func NewRage() *features.SimpleFeature {
    return features.NewSimple(
        features.WithRef(RageRef),  // Use the constant defined above
        features.FromSource(BarbarianClass),  // Also defined in this package
        features.AtLevel(1),
        features.OnApply(func(f *features.SimpleFeature, bus events.EventBus) error {
            // Actual rage implementation
            return nil
        }),
    )
}

// BarbarianClass is the source for barbarian features
var BarbarianClass = &core.Source{
    Category: core.SourceClass,
    Name:     "barbarian",
}
```

## Example: Poisoned

```go
// rulebooks/dnd5e/conditions/conditions.go
package conditions

import (
    "github.com/KirkDiggler/rpg-toolkit/core"
    "github.com/KirkDiggler/rpg-toolkit/mechanics/conditions"
)

// PoisonedRef is the identifier for the Poisoned condition
var PoisonedRef = core.MustNewRef(core.RefInput{
    Module: "dnd5e",
    Type:   "condition",
    Value:  "poisoned",
})

// NewPoisoned creates a new Poisoned condition
func NewPoisoned(target core.Entity) *conditions.SimpleCondition {
    return conditions.NewSimple(
        conditions.WithRef(PoisonedRef),
        conditions.WithTarget(target),
        conditions.OnApply(func(c *conditions.SimpleCondition, bus events.EventBus) error {
            // Poisoned effects: disadvantage on attacks and ability checks
            return nil
        }),
    )
}
```

## Example: Fireball

```go
// rulebooks/dnd5e/spells/fireball.go
package spells

import (
    "github.com/KirkDiggler/rpg-toolkit/core"
    "github.com/KirkDiggler/rpg-toolkit/mechanics/spells"
)

// FireballRef is the identifier for Fireball
var FireballRef = core.MustNewRef(core.RefInput{
    Module: "dnd5e",
    Type:   "spell",
    Value:  "fireball",
})

// NewFireball creates a new Fireball spell
func NewFireball() *spells.SimpleSpell {
    return spells.NewSimple(
        spells.WithRef(FireballRef),
        spells.AtLevel(3),
        spells.OnCast(func(s *spells.SimpleSpell, targets []core.Entity) error {
            // 8d6 fire damage in 20ft radius
            return nil
        }),
    )
}
```

## Usage in Game Server

```go
// In the game server
import (
    "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/classes/barbarian"
    "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/conditions"
    "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/spells"
)

// Give barbarian rage at level 1
character.AddFeature(barbarian.NewRage())

// Check if they have rage
if character.HasFeature(barbarian.RageRef) {
    // Can use rage
}

// Apply poisoned condition
character.AddCondition(conditions.NewPoisoned(character))

// Cast fireball
spells.NewFireball().Cast(targets)
```

## What Else Can Be Streamlined?

1. **Damage Types**
```go
// mechanics/damage/types.go
var (
    Fire = core.MustNewRef(core.RefInput{Module: "core", Type: "damage", Value: "fire"})
    Cold = core.MustNewRef(core.RefInput{Module: "core", Type: "damage", Value: "cold"})
)
```

2. **Skills**
```go
// rulebooks/dnd5e/skills/skills.go
var (
    Acrobatics = core.MustNewRef(core.RefInput{Module: "dnd5e", Type: "skill", Value: "acrobatics"})
    Athletics = core.MustNewRef(core.RefInput{Module: "dnd5e", Type: "skill", Value: "athletics"})
)
```

3. **Abilities**
```go
// rulebooks/dnd5e/abilities/abilities.go
var (
    Strength = core.MustNewRef(core.RefInput{Module: "dnd5e", Type: "ability", Value: "strength"})
    Dexterity = core.MustNewRef(core.RefInput{Module: "dnd5e", Type: "ability", Value: "dexterity"})
)
```

## The Pattern

**Implementation, identity, AND loading live together.** When you implement Rage, you export:
- `RageRef` - The identifier
- `NewRage()` - The factory
- `LoadRageFromData()` - The loader
- Maybe even `AvailableFeatures` - What can be loaded

This way:
- No separate refs package to maintain
- Identity is right next to behavior
- Loading logic lives with the feature that knows its own data
- Import what you use
- Clear ownership of each feature/spell/condition

## What The Package Exposes

```go
// rulebooks/dnd5e/classes/barbarian/features.go
package barbarian

// AvailableFeatures lists all features this package can load
var AvailableFeatures = []*core.Ref{
    RageRef,
    FrenzyRef,
    MindlessRageRef,
}

// LoadFromData loads any barbarian feature from data
func LoadFromData(ref *core.Ref, data map[string]interface{}) (features.Feature, error) {
    switch ref {
    case RageRef:
        return LoadRageFromData(data)
    case FrenzyRef:
        return LoadFrenzyFromData(data)
    default:
        return nil, fmt.Errorf("unknown barbarian feature: %s", ref)
    }
}
```