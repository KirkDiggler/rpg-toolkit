# Simple Feature Data Persistence

## The Real Problem

We need to save/load the COMPLETE state:
- Rage: uses remaining, is active, turns active
- Poisoned: remaining ticks, save DC, who poisoned you
- Spell slots: how many left at each level

## Option 1: JSON RawMessage (Simplest)

```go
// What gets saved to database
type CharacterData struct {
    ID       string                   `json:"id"`
    Name     string                   `json:"name"`
    Features []json.RawMessage        `json:"features"`  // Just raw JSON
}

// Each feature knows its own data format
type RageData struct {
    Ref           string `json:"ref"`  // "dnd5e:feature:rage"
    Level         int    `json:"level"`
    UsesRemaining int    `json:"uses_remaining"`
    MaxUses       int    `json:"max_uses"`
    IsActive      bool   `json:"is_active"`
    TurnsActive   int    `json:"turns_active"`
}

// ToData returns the complete state
func (r *RageFeature) ToData() json.RawMessage {
    data := RageData{
        Ref:           RageRef.String(),
        Level:         r.Level(),
        UsesRemaining: r.usesRemaining,
        MaxUses:       r.maxUses,
        IsActive:      r.isActive,
        TurnsActive:   r.turnsActive,
    }
    bytes, _ := json.Marshal(data)
    return bytes
}

// Loading - peek at ref, then unmarshal
func LoadFeatureFromJSON(rawData json.RawMessage) (Feature, error) {
    // Peek at just the ref
    var peek struct {
        Ref string `json:"ref"`
    }
    if err := json.Unmarshal(rawData, &peek); err != nil {
        return nil, err
    }
    
    // Now unmarshal into the right type and load
    switch peek.Ref {
    case "dnd5e:feature:rage":
        var data RageData
        if err := json.Unmarshal(rawData, &data); err != nil {
            return nil, err
        }
        return barbarian.LoadRageFromData(data), nil
        
    case "dnd5e:condition:poisoned":
        var data PoisonedData
        if err := json.Unmarshal(rawData, &data); err != nil {
            return nil, err
        }
        return conditions.LoadPoisonedFromData(data), nil
    
    default:
        return nil, fmt.Errorf("unknown feature: %s", peek.Ref)
    }
}
```

## Option 2: Generic Feature Container

```go
// Feature with generic data type
type Feature[T any] interface {
    core.Entity
    Ref() *core.Ref
    Apply(bus events.EventBus) error
    Remove(bus events.EventBus) error
    ToData() T
    IsDirty() bool
}

// Rage is Feature[RageData]
type RageFeature struct {
    *features.SimpleFeature
    // ... fields
}

func (r *RageFeature) ToData() RageData {
    return RageData{
        Ref:           RageRef.String(),
        UsesRemaining: r.usesRemaining,
        // ... all the state
    }
}

func LoadRageFromData(data RageData) *RageFeature {
    // Recreate with full state
}
```

## Option 3: Just Use map[string]interface{} (Pragmatic)

```go
// Feature data is just a map
type FeatureData map[string]interface{}

func (r *RageFeature) ToData() FeatureData {
    return FeatureData{
        "ref":            RageRef.String(),
        "level":          r.Level(),
        "uses_remaining": r.usesRemaining,
        "max_uses":       r.maxUses,
        "is_active":      r.isActive,
        "turns_active":   r.turnsActive,
    }
}

func LoadFeatureFromData(data FeatureData) (Feature, error) {
    ref := data["ref"].(string)
    
    switch ref {
    case "dnd5e:feature:rage":
        return barbarian.LoadRageFromData(data), nil
    case "dnd5e:condition:poisoned":
        return conditions.LoadPoisonedFromData(data), nil
    default:
        return nil, fmt.Errorf("unknown feature: %s", ref)
    }
}

// Each feature extracts what it needs
func LoadRageFromData(data FeatureData) *RageFeature {
    return &RageFeature{
        usesRemaining: data["uses_remaining"].(int),
        maxUses:       data["max_uses"].(int),
        isActive:      data["is_active"].(bool),
        turnsActive:   data["turns_active"].(int),
        // ...
    }
}
```

## What Actually Matters

As long as:
1. `ToData()` captures ALL the state
2. `LoadFromData(ToData())` recreates the exact same feature
3. It's JSON-serializable for the database

Then we're good!

## My Recommendation: Option 1 (JSON RawMessage)

```go
// Simple, clean, type-safe where it matters
type CharacterData struct {
    Features []json.RawMessage `json:"features"`
}

// Save
for _, feature := range character.features {
    charData.Features = append(charData.Features, feature.ToData())
}

// Load
for _, rawFeature := range charData.Features {
    feature, _ := LoadFeatureFromJSON(rawFeature)
    character.AddFeature(feature)
}
```

Why this works:
- **Complete state**: Each feature's ToData includes everything
- **Type safety**: RageData, PoisonedData are proper structs
- **Database friendly**: It's just JSON
- **Simple**: Peek at ref, unmarshal to right type, load

The key insight: **We don't need a fancy type system. We just need ToData() output to go into LoadFromData() and get back where we were.**