# Features Level Up Too!

## The Missing Piece

Features aren't static - they improve with character level:
- Rage damage: +2 (level 1) → +3 (level 9) → +4 (level 16)
- Sneak Attack: 1d6 (level 1) → 2d6 (level 3) → 3d6 (level 5)
- Bardic Inspiration: d6 (level 1) → d8 (level 5) → d10 (level 10)

## The Interface Should Support This

```go
type Feature interface {
    core.Entity
    Ref() *core.Ref
    Apply(events.EventBus) error
    Remove(events.EventBus) error
    
    // Persistence
    ToJSON() json.RawMessage
    IsDirty() bool
    MarkClean()
    
    // MISSING: Level progression!
    UpdateLevel(newLevel int) error  // <-- This is what we need!
}
```

## How It Would Work

```go
// When character levels up
func (c *Character) LevelUp(newLevel int) error {
    c.Level = newLevel
    
    // Update all existing features
    for _, feature := range c.features {
        if err := feature.UpdateLevel(newLevel); err != nil {
            return err
        }
        feature.MarkDirty() // Save the updated state
    }
    
    // Then check for NEW features at this level
    // (from progression tables)
    
    return c.SaveIfDirty()
}

// Rage implements UpdateLevel
func (r *RageFeature) UpdateLevel(newLevel int) error {
    oldDamage := r.getRageDamageBonus(r.level)
    newDamage := r.getRageDamageBonus(newLevel)
    
    if oldDamage != newDamage {
        log.Printf("Rage damage increased: +%d → +%d", oldDamage, newDamage)
        r.dirty = true
    }
    
    // Update uses per day
    oldUses := r.maxUses
    r.maxUses = calculateRageUses(newLevel)
    if r.maxUses > oldUses {
        r.usesRemaining += (r.maxUses - oldUses) // Grant new uses
        log.Printf("Rage uses increased: %d → %d", oldUses, r.maxUses)
        r.dirty = true
    }
    
    r.level = newLevel
    return nil
}
```

## The Complete Data Flow

```go
// Feature data includes current level
type RageData struct {
    Ref           string `json:"ref"`
    Level         int    `json:"level"`        // Current level
    CharLevel     int    `json:"char_level"`   // Character's level
    UsesRemaining int    `json:"uses_remaining"`
    MaxUses       int    `json:"max_uses"`
}

// When loading from database
func LoadFromGameContext(ctx GameContext, charID string) (*Character, error) {
    data := database.Get(charID)
    char := &Character{Level: data.Level}
    
    for _, featJSON := range data.Features {
        feature := LoadFeatureFromJSON(featJSON)
        
        // Check if feature needs updating
        if feature.Level() < char.Level {
            feature.UpdateLevel(char.Level)
            feature.MarkDirty()
        }
        
        char.features = append(char.features, feature)
    }
    
    return char, nil
}
```

## Or Even Simpler: Features Calculate Based on Character Level

```go
// Features could just reference character level
type RageFeature struct {
    *features.SimpleFeature
    character     *Character  // Reference to owner
    usesRemaining int
}

func (r *RageFeature) getRageDamageBonus() int {
    // Calculate based on character's current level
    switch {
    case r.character.Level >= 16:
        return 4
    case r.character.Level >= 9:
        return 3
    default:
        return 2
    }
}

// No UpdateLevel needed - features auto-scale!
```

## The Real Question

Should features:
1. Track their own level and need UpdateLevel()?
2. Reference character level and auto-scale?
3. Be immutable and get replaced at level up?

```go
// Option 3: Replace features at level up
func (c *Character) LevelUp(newLevel int) {
    // Remove old rage
    c.RemoveFeature(barbarian.RageRef)
    
    // Add new rage at new level
    newRageData := RageData{
        Ref:           "dnd5e:feature:rage",
        Level:         newLevel,
        UsesRemaining: calculateRageUses(newLevel),
        MaxUses:       calculateRageUses(newLevel),
    }
    c.AddFeature(LoadRageFromData(newRageData))
}
```

## It Really Is Data All The Way Down

You're right - if features change with level, that's a core behavior that should be in the interface. The question is HOW they change:
- Mutable (UpdateLevel)
- Calculated (reference char level)
- Immutable (replace at level up)

All three could work!