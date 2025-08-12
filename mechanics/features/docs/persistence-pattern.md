# Persistence Pattern

## The ToJSON/LoadFromJSON Pattern

This pattern provides a consistent way to save and load state for features, conditions, resources, and other game objects.

## Core Concepts

### 1. Everything Persistable Implements ToJSON()

```go
type Persistable interface {
    ToJSON() json.RawMessage
    IsDirty() bool
    MarkClean()
}
```

### 2. Internal vs External APIs

- **ToData()** - Returns typed struct (internal use)
- **ToJSON()** - Returns json.RawMessage (for persistence)
- **LoadFromData(TypedData)** - Takes typed struct
- **LoadFromJSON(json.RawMessage)** - Takes JSON bytes

```go
// Internal - type safe
func (r *RageFeature) ToData() RageData {
    return RageData{
        UsesRemaining: r.usesRemaining,
        // ... typed fields
    }
}

// External - for database
func (r *RageFeature) ToJSON() json.RawMessage {
    bytes, _ := json.Marshal(r.ToData())
    return bytes
}
```

### 3. Loading Uses Simple Switch

```go
func loadFeature(data json.RawMessage) (Feature, error) {
    // Peek at ref to determine type
    var peek struct {
        Ref string `json:"ref"`
    }
    json.Unmarshal(data, &peek)
    
    // Switch on ref - no magic registration
    switch peek.Ref {
    case "dnd5e:feature:rage":
        return barbarian.LoadRageFromJSON(data)
    case "dnd5e:condition:poisoned":
        return conditions.LoadPoisonedFromJSON(data)
    default:
        return nil, fmt.Errorf("unknown: %s", peek.Ref)
    }
}
```

## What Uses This Pattern

- **Features** - Rage uses, active state
- **Conditions** - Poison ticks, save DC, source
- **Resources** - Spell slots used, max values
- **Items** - Charges, attunement state
- **Effects** - Blessed targets, duration
- **Position** - Room, coordinates, facing

## Database Storage

```sql
-- SQL databases store JSON as TEXT
CREATE TABLE character_state (
    character_id UUID,
    features_json TEXT,      -- Array of feature JSON
    conditions_json TEXT,    -- Array of condition JSON
    updated_at TIMESTAMP
);

-- Or normalized
CREATE TABLE character_features (
    id UUID PRIMARY KEY,
    character_id UUID,
    feature_json TEXT,       -- Individual feature JSON
    dirty BOOLEAN DEFAULT FALSE
);
```

## Dirty Tracking for Efficiency

```go
func (c *Character) SaveIfDirty() error {
    for _, feat := range c.features {
        if feat.IsDirty() {
            db.UpdateFeature(c.ID, feat.ToJSON())
            feat.MarkClean()
        }
    }
    return nil
}
```

## Complete Example: Poisoned Condition

```go
// PoisonedData is the saved state
type PoisonedData struct {
    Ref        string `json:"ref"`
    TicksLeft  int    `json:"ticks_left"`
    SaveDC     int    `json:"save_dc"`
    SourceID   string `json:"source_id"`
}

// ToJSON for persistence
func (p *PoisonedCondition) ToJSON() json.RawMessage {
    data := PoisonedData{
        Ref:       "dnd5e:condition:poisoned",
        TicksLeft: p.ticksLeft,
        SaveDC:    p.saveDC,
        SourceID:  p.source.GetID(),
    }
    bytes, _ := json.Marshal(data)
    return bytes
}

// LoadPoisonedFromJSON recreates from saved state
func LoadPoisonedFromJSON(bytes json.RawMessage) (*PoisonedCondition, error) {
    var data PoisonedData
    if err := json.Unmarshal(bytes, &data); err != nil {
        return nil, err
    }
    
    return &PoisonedCondition{
        ticksLeft: data.TicksLeft,
        saveDC:    data.SaveDC,
        // ... reconstruct
    }, nil
}
```

## Benefits

1. **Works everywhere** - Any database can store JSON/text
2. **Type safety internally** - Use structs, not maps
3. **Graceful degradation** - Skip unknown types
4. **Efficient saves** - Only save dirty objects
5. **Simple to understand** - Just switch on ref