# Feature Data Persistence Pattern

## The Core Interfaces

```go
// mechanics/features/feature.go
package features

// Feature is the runtime interface
type Feature interface {
    core.Entity
    Ref() *core.Ref
    Apply(bus events.EventBus) error
    Remove(bus events.EventBus) error
    
    // Data persistence
    ToData() FeatureData
    IsDirty() bool
    MarkClean()
}

// FeatureData is the common interface for all feature data
type FeatureData interface {
    GetRef() *core.Ref
    GetType() string  // Helps with loading the right implementation
}
```

## Character Data Structure

```go
// CharacterData is what gets saved/loaded
type CharacterData struct {
    ID       string         `json:"id"`
    Name     string         `json:"name"`
    Class    string         `json:"class"`
    Level    int            `json:"level"`
    Features []FeatureData  `json:"features"`  // Polymorphic feature data
}
```

## Loading Features - Just Switch on Ref

```go
// rulebooks/dnd5e/features.go
package dnd5e

import (
    "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/classes/barbarian"
    "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/classes/fighter"
    "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/classes/rogue"
)

// LoadFeatureFromData loads any D&D 5e feature from its data
func LoadFeatureFromData(data features.FeatureData) (features.Feature, error) {
    ref := data.GetRef()
    
    // We know all our features - just switch
    switch ref.String() {
    case "dnd5e:feature:rage":
        if rageData, ok := data.(barbarian.RageData); ok {
            return barbarian.LoadRageFromData(rageData), nil
        }
        
    case "dnd5e:feature:second_wind":
        if swData, ok := data.(fighter.SecondWindData); ok {
            return fighter.LoadSecondWindFromData(swData), nil
        }
        
    case "dnd5e:feature:sneak_attack":
        if saData, ok := data.(rogue.SneakAttackData); ok {
            return rogue.LoadSneakAttackFromData(saData), nil
        }
        
    case "dnd5e:feature:frenzy":
        if frenzyData, ok := data.(barbarian.FrenzyData); ok {
            return barbarian.LoadFrenzyFromData(frenzyData), nil
        }
    
    // ... more features as we implement them
    
    default:
        return nil, fmt.Errorf("unknown feature: %s", ref)
    }
    
    return nil, fmt.Errorf("data type mismatch for %s", ref)
}
```

## Rage with Proper Data Types

```go
// rulebooks/dnd5e/classes/barbarian/rage.go
package barbarian

// RageData implements FeatureData
type RageData struct {
    Ref           string `json:"ref"`
    Type          string `json:"type"`  // Always "barbarian.rage"
    Level         int    `json:"level"`
    UsesRemaining int    `json:"uses_remaining"`
    MaxUses       int    `json:"max_uses"`
    IsActive      bool   `json:"is_active"`
    TurnsActive   int    `json:"turns_active"`
}

func (d RageData) GetRef() *core.Ref {
    return core.MustNewRef(core.RefInput{
        Module: "dnd5e",
        Type:   "feature",
        Value:  "rage",
    })
}

func (d RageData) GetType() string {
    return "barbarian.rage"
}

// RageFeature tracks its dirty state
type RageFeature struct {
    *features.SimpleFeature
    usesRemaining int
    maxUses       int
    isActive      bool
    turnsActive   int
    dirty         bool  // Track if state changed
}

func (r *RageFeature) IsDirty() bool {
    return r.dirty
}

func (r *RageFeature) MarkClean() {
    r.dirty = false
}

func (r *RageFeature) ToData() features.FeatureData {
    return RageData{
        Ref:           r.Ref().String(),
        Type:          "barbarian.rage",
        Level:         r.Level(),
        UsesRemaining: r.usesRemaining,
        MaxUses:       r.maxUses,
        IsActive:      r.isActive,
        TurnsActive:   r.turnsActive,
    }
}

// Any state change marks as dirty
func (r *RageFeature) startRage(ctx context.Context, e events.Event, bus events.EventBus) error {
    // ... existing logic ...
    r.usesRemaining--
    r.isActive = true
    r.dirty = true  // Mark dirty!
    // ...
}

// No registration needed - the dnd5e package knows about rage!
```

## Character Loading and Saving

```go
// game/character.go
package game

type Character struct {
    core.Entity
    features []features.Feature
    eventBus events.EventBus
}

// LoadCharacterFromData creates a character from saved data
func LoadCharacterFromData(data CharacterData, bus events.EventBus) (*Character, error) {
    char := &Character{
        eventBus: bus,
        features: make([]features.Feature, 0, len(data.Features)),
    }
    
    // Load all features
    for _, featureData := range data.Features {
        feature, err := features.LoadFromData(featureData)
        if err != nil {
            log.Printf("Failed to load feature %s: %v", featureData.GetRef(), err)
            continue  // Skip unknown features
        }
        
        char.features = append(char.features, feature)
        feature.Apply(bus)  // Activate it
        feature.MarkClean()  // Just loaded, not dirty
    }
    
    return char, nil
}

// SaveIfDirty checks for dirty features and saves them
func (c *Character) SaveIfDirty() error {
    var dirtyFeatures []features.FeatureData
    
    for _, feature := range c.features {
        if feature.IsDirty() {
            dirtyFeatures = append(dirtyFeatures, feature.ToData())
            feature.MarkClean()
        }
    }
    
    if len(dirtyFeatures) > 0 {
        // Only save what changed
        return c.saveFeatures(dirtyFeatures)
    }
    
    return nil
}

// Called at end of turn
func (c *Character) EndTurn() error {
    // Publish turn end event
    event := events.NewGameEvent("turn.end", c.Entity, nil)
    c.eventBus.Publish(context.Background(), event)
    
    // Save any dirty features
    return c.SaveIfDirty()
}
```

## JSON Marshaling with Type Info

```go
// Custom marshaling to handle polymorphic FeatureData
type featureDataWrapper struct {
    Type string          `json:"type"`
    Data json.RawMessage `json:"data"`
}

func MarshalFeatureData(fd features.FeatureData) ([]byte, error) {
    data, err := json.Marshal(fd)
    if err != nil {
        return nil, err
    }
    
    wrapper := featureDataWrapper{
        Type: fd.GetType(),
        Data: data,
    }
    
    return json.Marshal(wrapper)
}

func UnmarshalFeatureData(data []byte) (features.FeatureData, error) {
    var wrapper featureDataWrapper
    if err := json.Unmarshal(data, &wrapper); err != nil {
        return nil, err
    }
    
    // Use type to determine concrete type
    switch wrapper.Type {
    case "barbarian.rage":
        var rageData barbarian.RageData
        if err := json.Unmarshal(wrapper.Data, &rageData); err != nil {
            return nil, err
        }
        return rageData, nil
    // ... other types
    default:
        return nil, fmt.Errorf("unknown feature type: %s", wrapper.Type)
    }
}
```

## The Complete Flow

```go
// 1. Game starts - no registration needed!

// 2. Load character from database
var charData CharacterData
db.Load("character-123", &charData)

// 3. Create runtime character
character := LoadCharacterFromData(charData, eventBus)
// Each feature is now Applied and listening

// 4. During play, features change state
// Rage gets used, marks itself dirty

// 5. End of turn
character.EndTurn()
// Checks all features for IsDirty()
// Saves only the dirty ones

// 6. Character logs out
fullData := character.ToData()  // All features
db.Save("character-123", fullData)
```

## Key Insights

1. **FeatureData interface** - Common contract for all feature data
2. **Type field** - Tells us which loader to use
3. **Dirty tracking** - Only save what changed
4. **Registration pattern** - Each package registers its loaders
5. **Graceful degradation** - Skip features we don't know about

This gives you:
- Type safety (RageData, not map[string]interface{})
- Polymorphism ([]FeatureData can hold any feature)
- Efficiency (only save dirty features)
- Extensibility (new features just register themselves)