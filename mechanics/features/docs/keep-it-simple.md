# Keep It Simple - Features Are Features

## The Problem with "Everything is Actions"

Too abstract! We lose the structure that helps us organize and reason about the code.

## Keep Domain Concepts Clear

```go
// Character has concrete, typed collections
type Character struct {
    ID         string
    Name       string
    
    // Clear, organized, typed
    Features   []Feature    // Rage, Second Wind
    Spells     []Spell      // Fireball, Shield  
    Conditions []Condition  // Poisoned, Blessed
    Resources  []Resource   // Spell slots, rage uses
}
```

## Features Package Knows Features

```go
// mechanics/features/loader.go
func LoadFeatureFromJSON(data json.RawMessage) (Feature, error) {
    var peek struct {
        Ref string `json:"ref"`
    }
    json.Unmarshal(data, &peek)
    
    // Features package knows all features
    switch peek.Ref {
    case "dnd5e:feature:rage":
        var rageData RageData
        json.Unmarshal(data, &rageData)
        return LoadRageFromData(rageData), nil
        
    case "dnd5e:feature:second_wind":
        var swData SecondWindData
        json.Unmarshal(data, &swData)
        return LoadSecondWindFromData(swData), nil
        
    default:
        return nil, fmt.Errorf("unknown feature: %s", peek.Ref)
    }
}
```

## Feature Interface - Just What We Need

```go
type Feature interface {
    // Identity
    Ref() *core.Ref
    Name() string
    Description() string
    
    // Can it be activated?
    CanActivate() bool
    NeedsTarget() bool
    Activate(target core.Entity) error  // Optional target
    
    // State
    IsActive() bool
    GetRemainingUses() string  // "2/3 uses"
    
    // Event handling
    Apply(bus events.EventBus) error
    Remove(bus events.EventBus) error
    
    // Persistence
    ToJSON() json.RawMessage
    IsDirty() bool
    MarkClean()
}
```

## Loading Is Straightforward

```go
func LoadCharacterFromContext(ctx GameContext, charID string) (*Character, error) {
    var data CharacterData
    ctx.Database.Get(charID, &data)
    
    char := &Character{
        ID:   data.ID,
        Name: data.Name,
    }
    
    // Load features as features
    for _, featJSON := range data.Features {
        feature := LoadFeatureFromJSON(featJSON)
        char.Features = append(char.Features, feature)
        feature.Apply(ctx.EventBus)
    }
    
    // Load spells as spells
    for _, spellJSON := range data.Spells {
        spell := LoadSpellFromJSON(spellJSON)
        char.Spells = append(char.Spells, spell)
        spell.Apply(ctx.EventBus)
    }
    
    return char, nil
}
```

## Activation Is Simple

```go
// Player wants to activate something
func (c *Character) ActivateFeature(ref string, targetID string) error {
    // Find the feature
    for _, feature := range c.Features {
        if feature.Ref().String() == ref {
            // Get target if needed
            var target core.Entity
            if feature.NeedsTarget() {
                target = c.room.GetEntity(targetID)
                if target == nil {
                    return ErrInvalidTarget
                }
            }
            
            return feature.Activate(target)
        }
    }
    
    return ErrFeatureNotFound
}
```

## Why This Is Better

1. **Clear structure** - Features are features, spells are spells
2. **Type safety** - Not everything in a generic map
3. **Discoverable** - Easy to see what a character has
4. **Simple to reason about** - Feature → Activate → Effects
5. **Alpha-appropriate** - Ship something that works

## The Rage Implementation Stays Simple

```go
type RageFeature struct {
    *effects.Core
    level         int
    usesRemaining int
    isActive      bool
}

func (r *RageFeature) Activate(target core.Entity) error {
    // Target is ignored - rage affects self
    if r.isActive {
        return AlreadyActiveError()
    }
    
    if r.usesRemaining <= 0 {
        return NoUsesError()
    }
    
    r.usesRemaining--
    r.isActive = true
    r.dirty = true
    
    // Fire event
    event := events.NewGameEvent("feature.activated", r.owner, nil)
    event.Context().Set("feature", RageRef)
    
    return r.eventBus.Publish(context.Background(), event)
}
```

## For the Future

Maybe later we abstract to a generic action system, but for now:
- **Features are features**
- **Spells are spells**
- **They can be activated**
- **They apply effects through events**

Keep it simple, ship it, learn from real usage, THEN maybe generalize.

## The Success Metric Remains

**How simple is it to implement Rage?**

With this approach: Very simple! No abstract action system to understand, just:
1. Implement Feature interface
2. Handle Activate()
3. Subscribe to events
4. Done

That's alpha-ready!