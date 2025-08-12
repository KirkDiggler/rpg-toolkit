# Feature Data Loading Pattern

## The Insight

Instead of hardcoding feature creation, load them from data and get back the full Feature interface with `Apply()`.

## How It Works

```go
// mechanics/features/loader.go
package features

// LoadFromData creates a feature from saved data
func LoadFromData(data FeatureData) (Feature, error) {
    // Look up the feature implementation by its Ref
    factory, exists := registry[data.Ref]
    if !exists {
        return nil, fmt.Errorf("unknown feature: %s", data.Ref)
    }
    
    // Create the feature with saved state
    feature := factory(data)
    return feature, nil
}
```

## Character Loading

```go
// Loading a character from database/save file
func LoadCharacterFromContext(ctx GameContext) (*Character, error) {
    char := &Character{
        Entity:   ctx.GetEntity("character-id"),
        features: features.NewFeatureManager(),
        eventBus: ctx.GetEventBus(),
    }
    
    // Load feature data from storage
    featureDataList := ctx.LoadFeatureData(char.ID)
    
    for _, data := range featureDataList {
        // LoadFromData returns a Feature with Apply(), Remove(), etc.
        feature, err := features.LoadFromData(data)
        if err != nil {
            continue // Log error, skip unknown features
        }
        
        // Add to character
        char.features.Add(feature)
        
        // Apply to activate it
        feature.Apply(char.eventBus)
    }
    
    return char, nil
}
```

## The Registry Pattern

```go
// rulebooks/dnd5e/registry.go
package dnd5e

import (
    "github.com/KirkDiggler/rpg-toolkit/mechanics/features"
    "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/classes/barbarian"
)

func RegisterFeatures() {
    // Register all D&D 5e features
    features.Register(barbarian.RageRef, barbarian.NewRageFromData)
    features.Register(fighter.SecondWindRef, fighter.NewSecondWindFromData)
    features.Register(rogue.SneakAttackRef, rogue.NewSneakAttackFromData)
}
```

## Feature Factory with Data

```go
// rulebooks/dnd5e/classes/barbarian/features.go
package barbarian

// NewRageFromData creates Rage from saved data
func NewRageFromData(data features.FeatureData) features.Feature {
    return &RageFeature{
        SimpleFeature: features.NewSimple(
            features.WithRef(RageRef),
            features.FromSource(BarbarianClass),
            features.OnApply(func(f *features.SimpleFeature, bus events.EventBus) error {
                // Rage logic here
                return nil
            }),
        ),
        usesRemaining: data.GetInt("uses_remaining", 2), // Restore saved state
        isActive:      data.GetBool("is_active", false),
    }
}
```

## The FeatureData Structure

```go
// What gets saved/loaded
type FeatureData struct {
    Ref    string                 `json:"ref"`     // "dnd5e:feature:rage"
    Source string                 `json:"source"`  // "class:barbarian"
    Level  int                    `json:"level"`   // When acquired
    State  map[string]interface{} `json:"state"`   // Feature-specific data
}
```

## Complete Flow

```go
// 1. Game starts, register all features
dnd5e.RegisterFeatures()

// 2. Load character
char := LoadCharacterFromContext(ctx)
// This internally:
//   - Loads feature data from storage
//   - Calls LoadFromData for each
//   - Gets back Feature interfaces
//   - Calls Apply() on each

// 3. Features are now active and listening
// When damage happens, rage resistance auto-applies
// When turn starts, available features are shown
// etc.
```

## Why This Is Better

1. **Data-driven**: Features come from saves/database, not hardcoded
2. **Extensible**: New features just need to register themselves
3. **Clean interface**: `LoadFromData` returns `Feature` with `Apply()`
4. **State preservation**: Features can save/restore their state
5. **Modular**: Each rulebook registers its own features

## The Key Methods

```go
type Feature interface {
    core.Entity
    Ref() *core.Ref
    Apply(bus events.EventBus) error    // Activate feature
    Remove(bus events.EventBus) error   // Deactivate feature
    ToData() FeatureData                // Save state
}
```

So your character loading becomes:
```go
// Super clean!
for _, data := range savedFeatures {
    if feature, err := features.LoadFromData(data); err == nil {
        character.AddFeature(feature)
        feature.Apply(eventBus)
    }
}
```