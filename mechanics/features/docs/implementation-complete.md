# The Problem & Solution - Complete Implementation

## The Problem

The current Feature interface has **14 methods** to implement, making even simple features complex. Most methods are boilerplate that obscures the actual game logic.

## The Solution  

Embed `effects.Core` to handle common functionality, leaving features to focus on their unique behavior. Features become simple, event-driven components that know how to save/load themselves.

## Table of Contents
1. [Simplified Interface](#simplified-interface)
2. [Complete Rage Example](#complete-rage-example)
3. [Game Server Integration](#game-server-integration)
4. [Key Patterns](#key-patterns)

## Simplified Interface

```go
type Feature interface {
    // Identity
    core.Entity         // GetID(), GetType()
    Ref() *core.Ref     // Unique identifier
    
    // Display
    Name() string
    Description() string
    
    // Activation
    CanActivate() bool
    NeedsTarget() bool
    Activate(target core.Entity) error
    
    // State
    IsActive() bool
    GetRemainingUses() string
    
    // Events
    Apply(events.EventBus) error
    Remove(events.EventBus) error
    
    // Persistence
    ToJSON() json.RawMessage
    IsDirty() bool
    MarkClean()
}
```

That's it! No GetModifiers(), GetProficiencies(), GetEventListeners(), etc.

## Complete Rage Example

### Where Features Live

```
rulebooks/
└── dnd5e/
    └── classes/
        └── barbarian/
            ├── barbarian.go      // Class definition
            └── rage.go           // Rage implementation (below)
```

### Complete Rage Implementation

```go
// rulebooks/dnd5e/classes/barbarian/rage.go
package barbarian

import (
    "context"
    "encoding/json"
    "fmt"
    
    "github.com/KirkDiggler/rpg-toolkit/core"
    "github.com/KirkDiggler/rpg-toolkit/dice"
    "github.com/KirkDiggler/rpg-toolkit/events"
    "github.com/KirkDiggler/rpg-toolkit/mechanics/features"
)

// RageRef is the identifier - lives with implementation
var RageRef = core.MustNewRef(core.RefInput{
    Module: "dnd5e",
    Type:   "feature",
    Value:  "rage",
})

// Class source constant
var Class = &core.Source{
    Category: core.SourceClass,
    Name:     "barbarian",
}

// RageData represents saved state
type RageData struct {
    Ref           string `json:"ref"`
    Level         int    `json:"level"`
    UsesRemaining int    `json:"uses_remaining"`
    MaxUses       int    `json:"max_uses"`
    IsActive      bool   `json:"is_active"`
    TurnsActive   int    `json:"turns_active"`
}

// RageFeature extends SimpleFeature with rage-specific state
type RageFeature struct {
    *features.SimpleFeature
    usesRemaining int
    maxUses       int
    isActive      bool
    turnsActive   int
    dirty         bool
}

// NewRage creates a new Rage feature (used for character creation/level up)
// The game server doesn't call this - it loads from saved data
func NewRage(level int) *RageFeature {
    rage := &RageFeature{
        maxUses:       calculateRageUses(level),
        usesRemaining: calculateRageUses(level),
    }
    
    rage.SimpleFeature = features.NewSimple(
        features.WithRef(RageRef),
        features.FromSource(Class),
        features.AtLevel(level),
        features.OnApply(rage.apply),
        features.OnRemove(rage.remove),
    )
    
    return rage
}

// LoadRageFromData recreates from saved state
func LoadRageFromData(data RageData) *RageFeature {
    rage := &RageFeature{
        usesRemaining: data.UsesRemaining,
        maxUses:       data.MaxUses,
        isActive:      data.IsActive,
        turnsActive:   data.TurnsActive,
    }
    
    rage.SimpleFeature = features.NewSimple(
        features.WithRef(RageRef),
        features.FromSource(Class),
        features.AtLevel(data.Level),
        features.OnApply(rage.apply),
        features.OnRemove(rage.remove),
    )
    
    return rage
}

// LoadRageFromJSON loads from JSON
func LoadRageFromJSON(bytes json.RawMessage) (*RageFeature, error) {
    var data RageData
    if err := json.Unmarshal(bytes, &data); err != nil {
        return nil, err
    }
    return LoadRageFromData(data), nil
}

// ToData returns structured data
func (r *RageFeature) ToData() RageData {
    return RageData{
        Ref:           RageRef.String(),
        Level:         r.SimpleFeature.Level(),
        UsesRemaining: r.usesRemaining,
        MaxUses:       r.maxUses,
        IsActive:      r.isActive,
        TurnsActive:   r.turnsActive,
    }
}

// ToJSON returns JSON for persistence
func (r *RageFeature) ToJSON() json.RawMessage {
    bytes, _ := json.Marshal(r.ToData())
    return bytes
}

// IsDirty checks if state changed
func (r *RageFeature) IsDirty() bool {
    return r.dirty
}

// MarkClean clears dirty flag
func (r *RageFeature) MarkClean() {
    r.dirty = false
}

// apply sets up event subscriptions
func (r *RageFeature) apply(f *features.SimpleFeature, bus events.EventBus) error {
    // Subscribe to activation
    f.Subscribe(bus, "feature.activate", 100, func(ctx context.Context, e events.Event) error {
        featureRef, _ := e.Context().Get("feature_ref")
        if ref, ok := featureRef.(*core.Ref); ok && ref.Equals(RageRef) {
            return r.startRage(ctx, e, bus)
        }
        return nil
    })
    
    // Add damage resistance while raging
    f.Subscribe(bus, events.EventBeforeTakeDamage, 50, func(ctx context.Context, e events.Event) error {
        if !r.isActive || e.Target().GetID() != f.Owner().GetID() {
            return nil
        }
        
        damageType, _ := e.Context().Get("damage_type")
        if dt, ok := damageType.(string); ok {
            if dt == "slashing" || dt == "piercing" || dt == "bludgeoning" {
                damage, _ := e.Context().Get("damage")
                if dmg, ok := damage.(int); ok {
                    reducedDamage := dmg / 2
                    e.Context().Set("damage", reducedDamage)
                    r.dirty = true
                }
            }
        }
        return nil
    })
    
    // Add rage damage bonus
    f.Subscribe(bus, events.EventOnDamageRoll, 50, func(ctx context.Context, e events.Event) error {
        if !r.isActive || e.Source().GetID() != f.Owner().GetID() {
            return nil
        }
        
        if isStrengthMelee, _ := e.Context().Get("is_strength_melee"); isStrengthMelee == true {
            rageBonus := r.getRageDamageBonus(f.Level())
            e.Context().AddModifier(events.NewModifier(
                events.ModifierDamageBonus,
                "Rage",
                dice.NewFlat(rageBonus),
            ))
        }
        return nil
    })
    
    return nil
}

// remove cleans up
func (r *RageFeature) remove(f *features.SimpleFeature, bus events.EventBus) error {
    if r.isActive {
        r.endRage(context.Background(), bus)
    }
    return nil
}

// startRage begins a rage
func (r *RageFeature) startRage(ctx context.Context, e events.Event, bus events.EventBus) error {
    if r.isActive {
        return fmt.Errorf("already raging")
    }
    if r.usesRemaining <= 0 {
        return fmt.Errorf("no rage uses remaining")
    }
    
    r.usesRemaining--
    r.isActive = true
    r.turnsActive = 0
    r.dirty = true
    
    return nil
}

// endRage stops the current rage
func (r *RageFeature) endRage(ctx context.Context, bus events.EventBus) error {
    r.isActive = false
    r.turnsActive = 0
    r.dirty = true
    return nil
}

// getRageDamageBonus returns bonus by level
func (r *RageFeature) getRageDamageBonus(level int) int {
    switch {
    case level >= 16:
        return 4
    case level >= 9:
        return 3
    default:
        return 2
    }
}

// calculateRageUses returns max uses by level
func calculateRageUses(level int) int {
    switch {
    case level >= 20:
        return -1 // Unlimited
    case level >= 17:
        return 6
    case level >= 12:
        return 5
    case level >= 6:
        return 4
    case level >= 3:
        return 3
    default:
        return 2
    }
}
```

## Game Server Integration

### LoadFromGameContext

```go
// game/character.go
func LoadFromGameContext(ctx GameContext, characterID string) (*Character, error) {
    // 1. Load character data
    var data CharacterData
    if err := ctx.Database.Get(characterID, &data); err != nil {
        return nil, err
    }
    
    // 2. Create character
    char := &Character{
        ID:       data.ID,
        Name:     data.Name,
        eventBus: ctx.EventBus,
    }
    
    // 3. Load features
    for _, featJSON := range data.Features {
        feature, err := loadFeature(featJSON)
        if err != nil {
            continue // Skip unknown features
        }
        
        char.features = append(char.features, feature)
        feature.Apply(ctx.EventBus)
        feature.MarkClean()
    }
    
    return char, nil
}

// loadFeature uses simple switch on ref
func loadFeature(data json.RawMessage) (features.Feature, error) {
    var peek struct {
        Ref string `json:"ref"`
    }
    json.Unmarshal(data, &peek)
    
    switch peek.Ref {
    case "dnd5e:feature:rage":
        return barbarian.LoadRageFromJSON(data)
    case "dnd5e:feature:second_wind":
        return fighter.LoadSecondWindFromJSON(data)
    default:
        return nil, fmt.Errorf("unknown feature: %s", peek.Ref)
    }
}
```

### During Play

```go
// Player activates rage
func (c *Character) ActivateRage() error {
    activateEvent := events.NewGameEvent("feature.activate", c, nil)
    activateEvent.Context().Set("feature_ref", barbarian.RageRef)
    return c.eventBus.Publish(context.Background(), activateEvent)
}

// Taking damage - rage automatically applies
func (c *Character) TakeDamage(damage int, damageType string) {
    dmgEvent := events.NewGameEvent(events.EventBeforeTakeDamage, nil, c)
    dmgEvent.Context().Set("damage", damage)
    dmgEvent.Context().Set("damage_type", damageType)
    
    c.eventBus.Publish(context.Background(), dmgEvent)
    
    // Rage reduced damage automatically if active
    finalDamage := dmgEvent.Context().Get("damage").(int)
    c.HP -= finalDamage
}
```

### Saving

```go
// End of turn - save dirty features
func (c *Character) EndTurn() error {
    for _, feat := range c.features {
        if feat.IsDirty() {
            c.database.UpdateFeature(c.ID, feat.ToJSON())
            feat.MarkClean()
        }
    }
    return nil
}

// On logout - save everything
func (c *Character) SaveComplete() error {
    data := CharacterData{
        ID:   c.ID,
        Name: c.Name,
    }
    
    for _, feat := range c.features {
        data.Features = append(data.Features, feat.ToJSON())
        feat.MarkClean()
    }
    
    return c.database.Save(c.ID, data)
}
```

## Persistence Pattern

### The ToJSON/LoadFromJSON Pattern

```go
// Feature interface exposes ToJSON for persistence
type Feature interface {
    core.Entity
    Ref() *core.Ref
    Apply(events.EventBus) error
    Remove(events.EventBus) error
    
    // Persistence
    ToJSON() json.RawMessage
    IsDirty() bool
    MarkClean()
}
```

### When to Use What

- **ToData()** - Internal API, returns typed struct (RageData)
- **ToJSON()** - External API, returns json.RawMessage for database
- **LoadFromData()** - Takes typed struct, used internally
- **LoadFromJSON()** - Takes JSON, used when loading from database

### Database Storage

```sql
-- JSON stored as TEXT in SQL
CREATE TABLE character_features (
    character_id UUID,
    feature_json TEXT,
    updated_at TIMESTAMP
);
```

## Key Patterns

### 1. Features Own Everything
- Ref constant (RageRef) lives with implementation
- Loading function lives with implementation  
- No central registry needed

### 2. Simple Switch for Loading
```go
switch peek.Ref {
case "dnd5e:feature:rage":
    return barbarian.LoadRageFromJSON(data)
// ... other features
}
```

### 3. Event-Driven Behavior
- Features subscribe to events in Apply()
- No need to check "is raging?" everywhere
- Automatic interaction through event bus

### 4. Dirty Tracking for Efficiency
- Only save what changed
- Mark dirty when state changes
- SaveIfDirty() at end of turn

### Success Metric

**How simple is it to implement Rage?**

Answer: Very simple! The complexity is in the game logic (damage resistance, bonus damage), not the framework. No 14-method interface to implement, just:
- Extend SimpleFeature
- Add your state
- Write Apply() to subscribe to events
- Implement ToJSON()/LoadFromJSON()

That's it!