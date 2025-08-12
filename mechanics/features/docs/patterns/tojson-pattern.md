# The ToJSON() Pattern

## Everything That Needs Persistence

This pattern applies to MANY things in our toolkit:

```go
// Common pattern for anything that needs saving
type Persistable interface {
    ToJSON() json.RawMessage
    IsDirty() bool
    MarkClean()
}
```

## What Will Use This Pattern

### 1. Features
```go
func (r *RageFeature) ToJSON() json.RawMessage {
    data := RageData{
        Ref:           "dnd5e:feature:rage",
        UsesRemaining: r.usesRemaining,
        IsActive:      r.isActive,
        // ... complete state
    }
    bytes, _ := json.Marshal(data)
    return bytes
}
```

### 2. Conditions
```go
func (p *PoisonedCondition) ToJSON() json.RawMessage {
    data := PoisonedData{
        Ref:         "dnd5e:condition:poisoned",
        TicksLeft:   p.ticksLeft,
        SaveDC:      p.saveDC,
        Source:      p.source.GetID(),
        AppliedTurn: p.appliedTurn,
    }
    bytes, _ := json.Marshal(data)
    return bytes
}
```

### 3. Resources
```go
func (s *SpellSlots) ToJSON() json.RawMessage {
    data := SpellSlotsData{
        Ref:   "dnd5e:resource:spell_slots",
        Level: s.level,
        Max:   s.max,
        Used:  s.used,
    }
    bytes, _ := json.Marshal(data)
    return bytes
}
```

### 4. Items/Equipment
```go
func (i *MagicSword) ToJSON() json.RawMessage {
    data := ItemData{
        Ref:       "dnd5e:item:flame_tongue",
        Equipped:  i.equipped,
        Charges:   i.charges,
        Attuned:   i.attuned,
        AttunedTo: i.attunedTo,
    }
    bytes, _ := json.Marshal(data)
    return bytes
}
```

### 5. Active Effects/Buffs
```go
func (b *BlessEffect) ToJSON() json.RawMessage {
    data := EffectData{
        Ref:        "dnd5e:effect:bless",
        CasterID:   b.caster.GetID(),
        TargetIDs:  b.getTargetIDs(),
        TurnsLeft:  b.turnsLeft,
    }
    bytes, _ := json.Marshal(data)
    return bytes
}
```

### 6. Spatial Position
```go
func (p *Position) ToJSON() json.RawMessage {
    data := PositionData{
        RoomID: p.roomID,
        X:      p.x,
        Y:      p.y,
        Z:      p.z,
        Facing: p.facing,
    }
    bytes, _ := json.Marshal(data)
    return bytes
}
```

## Character Becomes Simple

```go
type CharacterData struct {
    ID        string              `json:"id"`
    Name      string              `json:"name"`
    
    // Everything is just JSON
    Features   []json.RawMessage  `json:"features"`
    Conditions []json.RawMessage  `json:"conditions"`
    Resources  []json.RawMessage  `json:"resources"`
    Equipment  []json.RawMessage  `json:"equipment"`
    Effects    []json.RawMessage  `json:"effects"`
    Position   json.RawMessage    `json:"position"`
}
```

## Loading Pattern is Consistent

```go
// Same pattern for everything
func LoadFeatureFromJSON(data json.RawMessage) (Feature, error) {
    return loadFromJSON("feature", data)
}

func LoadConditionFromJSON(data json.RawMessage) (Condition, error) {
    return loadFromJSON("condition", data)
}

func LoadResourceFromJSON(data json.RawMessage) (Resource, error) {
    return loadFromJSON("resource", data)
}

// Generic loader
func loadFromJSON(category string, data json.RawMessage) (interface{}, error) {
    // Peek at ref
    var peek struct {
        Ref string `json:"ref"`
    }
    json.Unmarshal(data, &peek)
    
    // Switch based on category and ref
    switch category {
    case "feature":
        switch peek.Ref {
        case "dnd5e:feature:rage":
            var rageData RageData
            json.Unmarshal(data, &rageData)
            return LoadRageFromData(rageData), nil
        // ...
        }
    case "condition":
        switch peek.Ref {
        case "dnd5e:condition:poisoned":
            var poisonData PoisonedData
            json.Unmarshal(data, &poisonData)
            return LoadPoisonedFromData(poisonData), nil
        // ...
        }
    }
}
```

## Database Storage

```sql
-- SQL works fine with JSON
CREATE TABLE character_state (
    character_id UUID PRIMARY KEY,
    state_json TEXT,  -- Just store the whole CharacterData as JSON
    updated_at TIMESTAMP
);

-- Or normalize it
CREATE TABLE character_features (
    id UUID PRIMARY KEY,
    character_id UUID,
    feature_json TEXT,  -- Individual feature JSON
    dirty BOOLEAN DEFAULT FALSE
);
```

## Why ToJSON() Is The Right Name

1. **Clear contract**: Returns JSON, not some abstract "data"
2. **Honest**: We're not pretending it's type-safe, it's JSON
3. **Familiar**: Matches JavaScript conventions
4. **Practical**: Every database can store strings/text

## The Pattern Scales

```go
// Everything that can be saved
type Persistable interface {
    ToJSON() json.RawMessage
    FromJSON(json.RawMessage) error  // Optional, if we want symmetry
    IsDirty() bool
    MarkClean()
}

// Now ANY system can save/load
func SaveDirtyObjects(objects []Persistable) {
    for _, obj := range objects {
        if obj.IsDirty() {
            db.Save(obj.ToJSON())
            obj.MarkClean()
        }
    }
}
```

This pattern will work for:
- Features
- Conditions  
- Resources
- Items
- Effects
- Positions
- Spells
- Abilities
- Even entire rooms/encounters

It's simple, it works, and every database can handle it!