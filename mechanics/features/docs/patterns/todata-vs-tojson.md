# ToData() vs ToJSON() - When to Use Which

## The Difference

### ToData() - Structured Data
```go
// Returns a typed struct
func (r *RageFeature) ToData() RageData {
    return RageData{
        Ref:           RageRef.String(),
        UsesRemaining: r.usesRemaining,
        IsActive:      r.isActive,
    }
}

// Load from typed struct
func LoadRageFromData(data RageData) *RageFeature {
    // data.UsesRemaining is typed as int
    // Compile-time safety
}
```

### ToJSON() - Serialized for Storage
```go
// Returns JSON bytes ready for database
func (r *RageFeature) ToJSON() json.RawMessage {
    data := r.ToData()  // First get structured data
    bytes, _ := json.Marshal(data)
    return bytes
}

// Load from JSON
func (r *RageFeature) LoadJSON(bytes json.RawMessage) error {
    var data RageData
    if err := json.Unmarshal(bytes, &data); err != nil {
        return err
    }
    *r = *LoadRageFromData(data)
    return nil
}
```

## The Pattern

**ToData() is internal, ToJSON() is for persistence:**

```go
type Feature interface {
    // Core functionality
    Apply(bus events.EventBus) error
    Remove(bus events.EventBus) error
    
    // For persistence
    ToJSON() json.RawMessage
    IsDirty() bool
    MarkClean()
}

// Implementation has both
type RageFeature struct {
    *SimpleFeature
    // ... fields
}

// ToData returns structured data (internal use)
func (r *RageFeature) ToData() RageData {
    return RageData{
        Ref:           RageRef.String(),
        UsesRemaining: r.usesRemaining,
        // ... all fields
    }
}

// ToJSON returns serialized data (for database)
func (r *RageFeature) ToJSON() json.RawMessage {
    bytes, _ := json.Marshal(r.ToData())
    return bytes
}

// LoadRageFromData creates from structured data
func LoadRageFromData(data RageData) *RageFeature {
    return &RageFeature{
        usesRemaining: data.UsesRemaining,
        // ... type safe
    }
}

// LoadRageFromJSON creates from JSON
func LoadRageFromJSON(bytes json.RawMessage) (*RageFeature, error) {
    var data RageData
    if err := json.Unmarshal(bytes, &data); err != nil {
        return nil, err
    }
    return LoadRageFromData(data), nil
}
```

## When to Use Which

### Use ToData()/FromData() when:
- Working within Go code
- Need compile-time type safety
- Passing data between functions
- Testing (easier to construct test data)

### Use ToJSON()/LoadJSON() when:
- Saving to database
- Sending over network
- Saving to file
- Need polymorphic storage ([]json.RawMessage)

## The Full Chain

```go
// Runtime → Storage
RageFeature → ToData() → RageData → json.Marshal → JSON → Database

// Storage → Runtime  
Database → JSON → json.Unmarshal → RageData → LoadFromData() → RageFeature
```

## Practical Example

```go
// The Feature interface only exposes ToJSON for persistence
type Feature interface {
    core.Entity
    Apply(events.EventBus) error
    Remove(events.EventBus) error
    ToJSON() json.RawMessage  // For saving
    IsDirty() bool
    MarkClean()
}

// But internally, implementations use ToData
type RageFeature struct {
    // ...
}

func (r *RageFeature) ToJSON() json.RawMessage {
    // ToData is internal, gives us type safety
    data := r.ToData()
    bytes, _ := json.Marshal(data)
    return bytes
}

// Character saves everything as JSON
type CharacterData struct {
    Features []json.RawMessage `json:"features"`
}

func (c *Character) Save() CharacterData {
    data := CharacterData{}
    for _, feat := range c.features {
        data.Features = append(data.Features, feat.ToJSON())
    }
    return data
}

// Loading switches on ref
func LoadCharacter(data CharacterData) *Character {
    char := &Character{}
    for _, featJSON := range data.Features {
        // Peek at ref
        var peek struct {
            Ref string `json:"ref"`
        }
        json.Unmarshal(featJSON, &peek)
        
        // Load with right function
        switch peek.Ref {
        case "dnd5e:feature:rage":
            feat, _ := barbarian.LoadRageFromJSON(featJSON)
            char.AddFeature(feat)
        // ...
        }
    }
    return char
}
```

## The Rule of Thumb

- **Internal API**: Use ToData()/LoadFromData() with typed structs
- **External API**: Use ToJSON()/LoadFromJSON() for persistence
- **Interface**: Only expose ToJSON() in the Feature interface
- **Implementation**: Use ToData() internally for type safety

This gives you:
- Type safety where it matters (internal)
- Flexibility where you need it (storage)
- Clean separation of concerns