# Features Usage Example: Registration and Triggering

## Loading a Character with Features

```go
// In your game server/character loading
package game

import (
    "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/classes/barbarian"
    "github.com/KirkDiggler/rpg-toolkit/mechanics/features"
)

type Character struct {
    core.Entity
    features *features.FeatureManager  // Manages all features
    eventBus events.EventBus
}

// LoadCharacterFromContext loads a character with their features
func LoadCharacterFromContext(ctx GameContext) (*Character, error) {
    char := &Character{
        Entity:   ctx.GetEntity("character-id"),
        features: features.NewFeatureManager(),
        eventBus: ctx.GetEventBus(),
    }
    
    // Load class features based on class and level
    if char.Class == "barbarian" && char.Level >= 1 {
        rage := barbarian.NewRage()
        
        // Register the feature with the character
        char.features.Add(rage)
        
        // Apply it to activate event subscriptions
        rage.Apply(char.eventBus)
    }
    
    return char, nil
}
```

## Feature Manager Pattern

```go
// mechanics/features/manager.go
package features

type FeatureManager struct {
    features map[string]Feature  // Keyed by Ref.String()
    active   map[string]bool
}

func (fm *FeatureManager) Add(feature Feature) error {
    key := feature.Ref().String()
    fm.features[key] = feature
    return nil
}

func (fm *FeatureManager) Has(ref *core.Ref) bool {
    _, exists := fm.features[ref.String()]
    return exists
}

func (fm *FeatureManager) Get(ref *core.Ref) (Feature, bool) {
    feat, exists := fm.features[ref.String()]
    return feat, exists
}
```

## Triggering Features

### 1. Passive Features (Always Active)

```go
// These apply their effects when loaded and stay active
rage := barbarian.NewRage()
rage.Apply(eventBus)  // Subscribes to events, applies modifiers
```

### 2. Activated Features (Player Choice)

```go
// Player wants to rage
func (c *Character) ActivateRage() error {
    rage, exists := c.features.Get(barbarian.RageRef)
    if !exists {
        return fmt.Errorf("character doesn't have rage")
    }
    
    // Check if can rage (has uses left, not already raging, etc)
    if !c.CanRage() {
        return fmt.Errorf("cannot rage right now")
    }
    
    // Trigger the rage
    rageEvent := events.NewGameEvent(
        "feature.activated",
        c.Entity,
        map[string]interface{}{
            "feature": barbarian.RageRef,
        },
    )
    
    return c.eventBus.Publish(ctx, rageEvent)
}
```

### 3. Triggered Features (React to Events)

```go
// rulebooks/dnd5e/classes/barbarian/features.go
func NewRage() *features.SimpleFeature {
    return features.NewSimple(
        features.WithRef(RageRef),
        features.OnApply(func(f *features.SimpleFeature, bus events.EventBus) error {
            // Subscribe to damage events while raging
            f.Subscribe(bus, events.EventBeforeDamage, 100, 
                func(ctx context.Context, e events.Event) error {
                    if !f.IsActive() {
                        return nil  // Not raging
                    }
                    
                    // Reduce physical damage by half
                    if dmg, ok := e.Data()["damage_type"].(string); ok {
                        if dmg == "slashing" || dmg == "piercing" || dmg == "bludgeoning" {
                            e.ModifyData("damage", func(val interface{}) interface{} {
                                if d, ok := val.(int); ok {
                                    return d / 2  // Resistance
                                }
                                return val
                            })
                        }
                    }
                    return nil
                })
            return nil
        }),
    )
}
```

## Complete Flow Example

```go
// Game combat loop
func HandleCombat(ctx GameContext) {
    barbarian := LoadCharacterFromContext(ctx)
    
    // Player decides to rage
    barbarian.ActivateRage()
    
    // Enemy attacks barbarian
    attackEvent := events.NewGameEvent(
        events.EventBeforeDamage,
        barbarian.Entity,
        map[string]interface{}{
            "damage": 20,
            "damage_type": "slashing",
        },
    )
    
    // Publish event - rage feature will automatically trigger
    ctx.EventBus.Publish(ctx, attackEvent)
    // Damage is now 10 due to rage resistance
}
```

## Different Trigger Patterns

```go
// 1. Manual activation (player choice)
character.ActivateFeature(barbarian.RageRef)

// 2. Automatic trigger (passive)
// Happens via event subscriptions set up in Apply()

// 3. Conditional trigger
func NewSecondWind() *features.SimpleFeature {
    return features.NewSimple(
        features.WithRef(SecondWindRef),
        features.OnApply(func(f *features.SimpleFeature, bus events.EventBus) error {
            // Only available when below half health
            f.Subscribe(bus, "turn.start", 100,
                func(ctx context.Context, e events.Event) error {
                    if char.HP < char.MaxHP/2 && f.CanUse() {
                        // Show as available action
                        e.AddAvailableAction(SecondWindRef)
                    }
                    return nil
                })
            return nil
        }),
    )
}
```

## Key Points

1. **Features register themselves** via `Apply()` which sets up event subscriptions
2. **Event bus handles triggering** - features react to events they care about
3. **FeatureManager tracks what's available** - check with `Has()`, activate with `Get()`
4. **Different patterns for different features**:
   - Passive: Always listening
   - Activated: Player triggers
   - Reactive: Responds to game state