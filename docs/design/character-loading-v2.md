# Character Loading Design V2: Actions and Effects

## Overview

Characters have a fixed structure (D&D 5e character always looks the same), but features and effects are loaded dynamically via refs. Features provide **actions** that when activated create **effects**. Everything at runtime is an effect on the event bus.

## Core Architecture

### Fixed Character Structure

```go
// Character data from database/API - fixed structure
type CharacterData struct {
    // Identity
    ID       string
    Name     string
    
    // Fixed D&D 5e fields
    Level        int
    Abilities    AbilityScores
    Proficiency  int
    MaxHP        int
    CurrentHP    int
    AC          int
    Speed       int
    
    // Variable content - loaded via refs
    Features      []FeatureData
    Proficiencies []ProficiencyData
    
    // State
    Resources map[string]ResourceState  // "rage_uses": {current: 2, max: 3}
}

// Feature data includes ref and any feature-specific data
type FeatureData struct {
    Ref    string                 // "dnd5e:feature:rage"
    Source string                 // "barbarian:1" 
    Config map[string]interface{} // Feature-specific configuration
}

type ResourceState struct {
    Current int
    Max     int
    Reset   string // "short_rest", "long_rest", "dawn"
}
```

### Module Loading Pattern

```go
// Game server knows what modules it supports
type GameServer struct {
    eventBus     *events.Bus
    charLoader   *CharacterLoader
}

func NewGameServer() *GameServer {
    // Create module registry
    registry := modules.NewRegistry()
    
    // Register D&D 5e module (always present)
    registry.Register("dnd5e", dnd5e.NewModule())
    
    // Register optional modules
    registry.Register("homebrew", homebrew.NewModule())
    registry.Register("critical-role", criticalrole.NewModule())
    
    return &GameServer{
        eventBus: events.NewBus(),
        charLoader: &CharacterLoader{
            registry: registry,
        },
    }
}
```

## Feature Loading Flow

### 1. Character Loader

```go
type CharacterLoader struct {
    registry *modules.Registry
}

func (cl *CharacterLoader) Load(data CharacterData) (*Character, error) {
    // Create character with fixed fields
    char := &Character{
        ID:          data.ID,
        Name:        data.Name,
        Level:       data.Level,
        Abilities:   data.Abilities,
        Proficiency: data.Proficiency,
        MaxHP:       data.MaxHP,
        CurrentHP:   data.CurrentHP,
        
        // Runtime collections
        features:      []Feature{},
        proficiencies: []Proficiency{},
        activeEffects: []Effect{},
        resources:     data.Resources,
    }
    
    // Load features via refs
    for _, featData := range data.Features {
        feature, err := cl.loadFeature(char, featData)
        if err != nil {
            log.Printf("Failed to load feature %s: %v", featData.Ref, err)
            continue // Skip unknown features
        }
        char.features = append(char.features, feature)
    }
    
    // Load proficiencies
    for _, profData := range data.Proficiencies {
        prof, err := cl.loadProficiency(char, profData)
        if err != nil {
            log.Printf("Failed to load proficiency %s: %v", profData.Ref, err)
            continue
        }
        char.proficiencies = append(char.proficiencies, prof)
    }
    
    return char, nil
}

func (cl *CharacterLoader) loadFeature(owner *Character, data FeatureData) (Feature, error) {
    ref, err := core.ParseRef(data.Ref)
    if err != nil {
        return nil, err
    }
    
    module, err := cl.registry.Get(ref.Module)
    if err != nil {
        return nil, fmt.Errorf("module not found: %s", ref.Module)
    }
    
    return module.LoadFeature(owner, ref, data)
}
```

### 2. Module Interface

```go
// Each module knows how to load its content
type Module interface {
    LoadFeature(owner *Character, ref *core.Ref, data FeatureData) (Feature, error)
    LoadProficiency(owner *Character, ref *core.Ref, data ProficiencyData) (Proficiency, error)
}
```

### 3. D&D 5e Module Implementation

```go
// rulebooks/dnd5e/module.go
package dnd5e

import (
    "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/features/rage"
    "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/features/second_wind"
    "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/features/fighting_style"
)

type Module struct {
    // Could have configuration
}

func NewModule() *Module {
    return &Module{}
}

func (m *Module) LoadFeature(owner *Character, ref *core.Ref, data FeatureData) (Feature, error) {
    switch ref.Value {
    case "rage":
        return rage.Load(owner, data)
    case "second_wind":
        return second_wind.Load(owner, data)
    case "fighting_style":
        return fighting_style.Load(owner, data)
    default:
        return nil, fmt.Errorf("unknown feature: %s", ref.Value)
    }
}
```

## Rage Feature Example

### Package Structure
```
rulebooks/dnd5e/
├── features/
│   ├── rage/
│   │   ├── rage.go          # Main feature implementation
│   │   ├── effects.go       # Rage-specific effects
│   │   └── rage_test.go
│   ├── second_wind/
│   │   └── second_wind.go
│   └── fighting_style/
│       └── fighting_style.go
```

### Rage Implementation

```go
// rulebooks/dnd5e/features/rage/rage.go
package rage

import (
    "github.com/KirkDiggler/rpg-toolkit/core"
    "github.com/KirkDiggler/rpg-toolkit/events"
    "github.com/KirkDiggler/rpg-toolkit/mechanics/features"
)

// RageFeature provides the rage action and manages rage state
type RageFeature struct {
    *features.SimpleFeature
    
    owner         *Character
    level         int
    usesRemaining int
    maxUses       int
    isActive      bool
    damageBonus   int
}

// Load creates a rage feature from data
func Load(owner *Character, data FeatureData) (*RageFeature, error) {
    // Calculate based on barbarian level
    level := owner.Level
    maxUses := calculateMaxRages(level)
    damageBonus := calculateRageDamage(level)
    
    // Get current uses from resources
    usesRemaining := maxUses
    if state, ok := owner.Resources["rage_uses"]; ok {
        usesRemaining = state.Current
    }
    
    rage := &RageFeature{
        SimpleFeature: features.NewSimpleFeature(features.SimpleFeatureConfig{
            ID:     fmt.Sprintf("rage_%s", owner.ID),
            Type:   "feature.rage",
            Source: core.ParseSource(data.Source),
        }),
        owner:         owner,
        level:         level,
        usesRemaining: usesRemaining,
        maxUses:       maxUses,
        damageBonus:   damageBonus,
        isActive:      false,
    }
    
    return rage, nil
}

// GetActions returns available actions from this feature
func (r *RageFeature) GetActions() []Action {
    if r.isActive {
        return []Action{
            &EndRageAction{feature: r},
        }
    }
    
    if r.usesRemaining > 0 {
        return []Action{
            &EnterRageAction{feature: r},
        }
    }
    
    return nil
}

// Activate is called when rage is activated
func (r *RageFeature) Activate(bus events.EventBus) error {
    if r.usesRemaining <= 0 {
        return ErrNoUsesRemaining
    }
    
    if r.isActive {
        return ErrAlreadyActive
    }
    
    // Create rage effects
    effects := r.createRageEffects()
    
    // Apply all effects to the bus
    for _, effect := range effects {
        if err := effect.Apply(bus); err != nil {
            return fmt.Errorf("apply rage effect: %w", err)
        }
        r.owner.AddActiveEffect(effect)
    }
    
    r.isActive = true
    r.usesRemaining--
    
    // Update resource tracking
    r.owner.UpdateResource("rage_uses", r.usesRemaining)
    
    // Publish rage activated event
    bus.Publish(context.Background(), &RageActivatedEvent{
        Source: r.owner,
    })
    
    return nil
}

// Deactivate ends rage
func (r *RageFeature) Deactivate(bus events.EventBus) error {
    if !r.isActive {
        return nil
    }
    
    // Remove all rage effects
    r.owner.RemoveEffectsBySource("rage")
    
    r.isActive = false
    
    // Publish rage ended event
    bus.Publish(context.Background(), &RageEndedEvent{
        Source: r.owner,
    })
    
    return nil
}

func (r *RageFeature) createRageEffects() []Effect {
    return []Effect{
        // Damage bonus to melee attacks
        &RageDamageBonus{
            source: "rage",
            owner:  r.owner,
            bonus:  r.damageBonus,
        },
        
        // Resistance to physical damage
        &RageResistance{
            source:      "rage",
            owner:       r.owner,
            damageTypes: []string{"bludgeoning", "piercing", "slashing"},
        },
        
        // Advantage on Strength checks and saves
        &RageStrengthAdvantage{
            source: "rage",
            owner:  r.owner,
        },
    }
}

func calculateMaxRages(level int) int {
    switch {
    case level < 3:
        return 2
    case level < 6:
        return 3
    case level < 12:
        return 4
    case level < 17:
        return 5
    case level < 20:
        return 6
    default:
        return -1 // Unlimited at level 20
    }
}

func calculateRageDamage(level int) int {
    switch {
    case level < 9:
        return 2
    case level < 16:
        return 3
    default:
        return 4
    }
}
```

### Rage Effects

```go
// rulebooks/dnd5e/features/rage/effects.go
package rage

// RageDamageBonus adds damage to melee attacks while raging
type RageDamageBonus struct {
    source string
    owner  *Character
    bonus  int
}

func (e *RageDamageBonus) Apply(bus events.EventBus) error {
    // Subscribe to damage roll events
    return bus.SubscribeFunc("event.on_damage_roll", 50, func(ctx context.Context, event events.Event) error {
        // Only apply to our owner's attacks
        if event.Source() != e.owner {
            return nil
        }
        
        // Only melee attacks
        attackType, _ := event.Context().Get("attack_type")
        if attackType != "melee" {
            return nil
        }
        
        // Add rage damage bonus
        event.Context().AddModifier(events.NewModifier(
            "rage",
            events.ModifierDamageBonus,
            events.NewRawValue(e.bonus, "rage damage"),
            100,
        ))
        
        return nil
    })
}

func (e *RageDamageBonus) Remove(bus events.EventBus) error {
    // Unsubscribe from events
    // This would need subscription tracking
    return nil
}

// RageResistance provides resistance to physical damage
type RageResistance struct {
    source      string
    owner       *Character
    damageTypes []string
}

func (e *RageResistance) Apply(bus events.EventBus) error {
    return bus.SubscribeFunc("event.before_take_damage", 50, func(ctx context.Context, event events.Event) error {
        // Only apply to damage against our owner
        if event.Target() != e.owner {
            return nil
        }
        
        damageType, _ := event.Context().Get("damage_type")
        for _, resisted := range e.damageTypes {
            if damageType == resisted {
                // Halve the damage
                damage, _ := event.Context().Get("damage")
                event.Context().Set("damage", damage.(int)/2)
                event.Context().Set("resistance_applied", true)
                break
            }
        }
        
        return nil
    })
}
```

## Character Activation

```go
// Character connects all features/proficiencies to the event bus
type Character struct {
    // ... fields ...
    
    features      []Feature
    proficiencies []Proficiency
    activeEffects []Effect
}

// Activate wires everything to the event bus
func (c *Character) Activate(bus events.EventBus) error {
    // Features can register handlers but aren't activated yet
    for _, f := range c.features {
        if err := f.Setup(bus); err != nil {
            return fmt.Errorf("setup feature %s: %w", f.GetID(), err)
        }
    }
    
    // Proficiencies always apply
    for _, p := range c.proficiencies {
        if err := p.Apply(bus); err != nil {
            return fmt.Errorf("apply proficiency %s: %w", p.GetID(), err)
        }
    }
    
    return nil
}

// GetAvailableActions returns all actions from features
func (c *Character) GetAvailableActions() []Action {
    var actions []Action
    
    for _, f := range c.features {
        if provider, ok := f.(ActionProvider); ok {
            actions = append(actions, provider.GetActions()...)
        }
    }
    
    return actions
}

// ExecuteAction performs an action (like entering rage)
func (c *Character) ExecuteAction(actionID string, bus events.EventBus) error {
    actions := c.GetAvailableActions()
    
    for _, action := range actions {
        if action.GetID() == actionID {
            return action.Execute(c, bus)
        }
    }
    
    return fmt.Errorf("action not found: %s", actionID)
}
```

## Complete Flow Example

```go
func main() {
    // 1. Game server setup
    server := NewGameServer()
    
    // 2. Load character data (from DB/API)
    charData := CharacterData{
        ID:          "barb_123",
        Name:        "Ragnar",
        Level:       5,
        Abilities:   AbilityScores{Strength: 18, Constitution: 16},
        Proficiency: 3,
        MaxHP:       55,
        CurrentHP:   55,
        
        Features: []FeatureData{
            {
                Ref:    "dnd5e:feature:rage",
                Source: "barbarian:1",
            },
            {
                Ref:    "dnd5e:feature:second_wind",
                Source: "fighter:1",
            },
        },
        
        Proficiencies: []ProficiencyData{
            {
                Ref:    "dnd5e:proficiency:weapon",
                Target: "greatsword",
            },
        },
        
        Resources: map[string]ResourceState{
            "rage_uses": {Current: 3, Max: 3},
            "second_wind_uses": {Current: 1, Max: 1},
        },
    }
    
    // 3. Load character (features created but not active)
    character, err := server.charLoader.Load(charData)
    if err != nil {
        log.Fatal(err)
    }
    
    // 4. Activate character (wire to event bus)
    if err := character.Activate(server.eventBus); err != nil {
        log.Fatal(err)
    }
    
    // 5. Character can now use actions
    actions := character.GetAvailableActions()
    // actions = [EnterRageAction, SecondWindAction]
    
    // 6. Player activates rage
    err = character.ExecuteAction("enter_rage", server.eventBus)
    // This creates and applies rage effects to the bus
    
    // 7. During combat, rage effects automatically apply
    attackEvent := &AttackEvent{
        Source: character,
        Target: goblin,
        Type:   "melee",
    }
    
    server.eventBus.Publish(context.Background(), attackEvent)
    // RageDamageBonus automatically adds +2 damage
    // Proficiency automatically adds proficiency bonus
}
```

## Key Design Points

1. **Character structure is fixed** - D&D 5e character always has same fields
2. **Features loaded via refs** - Module pattern for extensibility
3. **Features provide actions** - Not always active, activated by player choice
4. **Actions create effects** - Everything at runtime is an effect
5. **Effects wire to event bus** - Automatic participation in game events
6. **Clean package structure** - Each feature in its own package

## Benefits

- **Modular**: Easy to add new features/modules
- **Data-driven**: Features loaded from refs
- **Event-driven**: Effects automatically participate
- **Testable**: Each feature is isolated
- **Extensible**: Homebrew modules plug in same way

## Open Questions

1. **Effect lifecycle**: How long do effects last? Rounds/minutes/until rest?
2. **Effect stacking**: Can you have multiple rage effects?
3. **Subscription management**: How to track/remove event subscriptions?
4. **State persistence**: How to save character state back to DB?