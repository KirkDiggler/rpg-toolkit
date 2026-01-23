# Character Loading Design

## Overview

This document describes how a character is loaded from pure data into a runtime entity with working features, proficiencies, and effects. The character data is completely self-contained - no external race/class lookups needed at load time.

## Design Principles

1. **Self-Contained Data**: Character has all compiled values (no lookups needed)
2. **Ref-Based Loading**: Features/proficiencies identified by refs that route to module loaders
3. **Event-Driven Runtime**: Loaded entities wire themselves to event bus
4. **Module Agnostic**: Core loading doesn't know about specific features

## Data Structures

### Character Data Format

```json
{
  "id": "char_abc123",
  "name": "Ragnar the Bold",
  "level": 5,
  
  "core": {
    "abilities": {
      "strength": 18,
      "dexterity": 14,
      "constitution": 16,
      "intelligence": 10,
      "wisdom": 13,
      "charisma": 8
    },
    "proficiency_bonus": 3,
    "armor_class": 16,
    "max_hit_points": 55,
    "speed": 30,
    "size": "medium"
  },
  
  "current_state": {
    "hit_points": 45,
    "temp_hit_points": 0,
    "exhaustion": 0,
    "death_saves": {
      "successes": 0,
      "failures": 0
    }
  },
  
  "features": [
    {
      "ref": "dnd5e:feature:rage",
      "source": "barbarian:1",
      "state": {
        "uses_remaining": 2,
        "max_uses": 3
      }
    },
    {
      "ref": "dnd5e:feature:fighting_style",
      "source": "fighter:1",
      "config": {
        "style": "great_weapon_fighting"
      }
    },
    {
      "ref": "dnd5e:feature:second_wind",
      "source": "fighter:1",
      "state": {
        "uses_remaining": 1,
        "max_uses": 1
      }
    }
  ],
  
  "proficiencies": [
    {
      "ref": "dnd5e:proficiency:weapon_group",
      "source": "barbarian:1",
      "config": {
        "group": "martial_weapons"
      }
    },
    {
      "ref": "dnd5e:proficiency:weapon_specific",
      "source": "background:soldier",
      "config": {
        "weapon": "longsword"
      }
    },
    {
      "ref": "dnd5e:proficiency:skill",
      "source": "barbarian:1",
      "config": {
        "skill": "athletics",
        "expertise": false
      }
    },
    {
      "ref": "dnd5e:proficiency:armor",
      "source": "barbarian:1",
      "config": {
        "types": ["light", "medium", "shields"]
      }
    }
  ],
  
  "conditions": [
    {
      "ref": "dnd5e:condition:exhaustion",
      "source": "forced_march",
      "level": 1,
      "duration": null
    }
  ],
  
  "resources": {
    "spell_slots": {
      "1": {"max": 0, "used": 0},
      "2": {"max": 0, "used": 0}
    },
    "hit_dice": {
      "d12": {"max": 5, "used": 2}
    },
    "class_resources": {
      "rage": {"max": 3, "used": 1, "reset": "long_rest"},
      "second_wind": {"max": 1, "used": 0, "reset": "short_rest"}
    }
  }
}
```

## Loading Architecture

### Module Registry

```go
// Each module provides a loader
type ModuleLoader interface {
    // Check if this loader handles the ref
    CanLoad(ref *core.Ref) bool
    
    // Load feature from JSON
    LoadFeature(ref *core.Ref, data json.RawMessage) (features.Feature, error)
    
    // Load proficiency from JSON
    LoadProficiency(ref *core.Ref, data json.RawMessage) (proficiency.Proficiency, error)
    
    // Load condition from JSON
    LoadCondition(ref *core.Ref, data json.RawMessage) (conditions.Condition, error)
}

// Registry manages all module loaders
type LoaderRegistry struct {
    modules map[string]ModuleLoader
}

func (r *LoaderRegistry) Register(module string, loader ModuleLoader) {
    r.modules[module] = loader
}

func (r *LoaderRegistry) GetLoader(ref *core.Ref) (ModuleLoader, error) {
    loader, ok := r.modules[ref.Module]
    if !ok {
        return nil, fmt.Errorf("no loader for module: %s", ref.Module)
    }
    return loader, nil
}
```

### Character Loader

```go
type CharacterLoader struct {
    registry *LoaderRegistry
    eventBus events.EventBus
}

type CharacterData struct {
    ID           string          `json:"id"`
    Name         string          `json:"name"`
    Level        int             `json:"level"`
    Core         CoreStats       `json:"core"`
    CurrentState CurrentState    `json:"current_state"`
    Features     []FeatureData   `json:"features"`
    Proficiencies []ProficiencyData `json:"proficiencies"`
    Conditions   []ConditionData `json:"conditions"`
    Resources    Resources       `json:"resources"`
}

type FeatureData struct {
    Ref    string          `json:"ref"`
    Source string          `json:"source"`
    State  json.RawMessage `json:"state,omitempty"`
    Config json.RawMessage `json:"config,omitempty"`
}

func (cl *CharacterLoader) Load(data []byte) (*Character, error) {
    var charData CharacterData
    if err := json.Unmarshal(data, &charData); err != nil {
        return nil, fmt.Errorf("unmarshal character data: %w", err)
    }
    
    // Create base character with compiled values
    char := &Character{
        ID:    charData.ID,
        Name:  charData.Name,
        Level: charData.Level,
        
        // Copy all compiled stats
        Abilities:       charData.Core.Abilities,
        ProficiencyBonus: charData.Core.ProficiencyBonus,
        ArmorClass:      charData.Core.ArmorClass,
        MaxHitPoints:    charData.Core.MaxHitPoints,
        Speed:           charData.Core.Speed,
        Size:            charData.Core.Size,
        
        // Current state
        HitPoints:     charData.CurrentState.HitPoints,
        TempHitPoints: charData.CurrentState.TempHitPoints,
        Exhaustion:    charData.CurrentState.Exhaustion,
        DeathSaves:    charData.CurrentState.DeathSaves,
        
        // Resources
        Resources: charData.Resources,
        
        // Runtime collections
        features:      []features.Feature{},
        proficiencies: []proficiency.Proficiency{},
        conditions:    []conditions.Condition{},
    }
    
    // Load features
    for _, featData := range charData.Features {
        feature, err := cl.loadFeature(featData)
        if err != nil {
            return nil, fmt.Errorf("load feature %s: %w", featData.Ref, err)
        }
        char.features = append(char.features, feature)
    }
    
    // Load proficiencies
    for _, profData := range charData.Proficiencies {
        prof, err := cl.loadProficiency(profData)
        if err != nil {
            return nil, fmt.Errorf("load proficiency %s: %w", profData.Ref, err)
        }
        char.proficiencies = append(char.proficiencies, prof)
    }
    
    // Load active conditions
    for _, condData := range charData.Conditions {
        cond, err := cl.loadCondition(condData)
        if err != nil {
            return nil, fmt.Errorf("load condition %s: %w", condData.Ref, err)
        }
        char.conditions = append(char.conditions, cond)
    }
    
    return char, nil
}

func (cl *CharacterLoader) loadFeature(data FeatureData) (features.Feature, error) {
    ref, err := core.ParseRef(data.Ref)
    if err != nil {
        return nil, fmt.Errorf("parse ref: %w", err)
    }
    
    loader, err := cl.registry.GetLoader(ref)
    if err != nil {
        return nil, err
    }
    
    // Create full JSON for module loader
    fullData := map[string]interface{}{
        "ref":    data.Ref,
        "source": data.Source,
    }
    if data.State != nil {
        fullData["state"] = data.State
    }
    if data.Config != nil {
        fullData["config"] = data.Config
    }
    
    jsonData, _ := json.Marshal(fullData)
    return loader.LoadFeature(ref, jsonData)
}
```

## Module Implementation

### D&D 5e Module Loader

```go
type DnD5eLoader struct {
    // Could have internal registries if needed
}

func (d *DnD5eLoader) CanLoad(ref *core.Ref) bool {
    return ref.Module == "dnd5e"
}

func (d *DnD5eLoader) LoadFeature(ref *core.Ref, data json.RawMessage) (features.Feature, error) {
    switch ref.Type {
    case "feature":
        return d.loadGameFeature(ref, data)
    default:
        return nil, fmt.Errorf("unknown feature type: %s", ref.Type)
    }
}

func (d *DnD5eLoader) loadGameFeature(ref *core.Ref, data json.RawMessage) (features.Feature, error) {
    switch ref.Value {
    case "rage":
        return d.loadRage(data)
    case "second_wind":
        return d.loadSecondWind(data)
    case "fighting_style":
        return d.loadFightingStyle(data)
    default:
        return nil, fmt.Errorf("unknown feature: %s", ref.Value)
    }
}

func (d *DnD5eLoader) loadRage(data json.RawMessage) (features.Feature, error) {
    var rageData struct {
        Ref    string `json:"ref"`
        Source string `json:"source"`
        State  struct {
            UsesRemaining int `json:"uses_remaining"`
            MaxUses       int `json:"max_uses"`
        } `json:"state"`
    }
    
    if err := json.Unmarshal(data, &rageData); err != nil {
        return nil, err
    }
    
    rage := &RageFeature{
        SimpleFeature: features.NewSimpleFeature(features.SimpleFeatureConfig{
            ID:     fmt.Sprintf("rage_%s", uuid.New()),
            Type:   "feature.rage",
            Source: core.ParseSource(rageData.Source),
        }),
        usesRemaining: rageData.State.UsesRemaining,
        maxUses:       rageData.State.MaxUses,
    }
    
    return rage, nil
}

func (d *DnD5eLoader) LoadProficiency(ref *core.Ref, data json.RawMessage) (proficiency.Proficiency, error) {
    switch ref.Value {
    case "weapon_group":
        return d.loadWeaponGroupProficiency(data)
    case "weapon_specific":
        return d.loadWeaponProficiency(data)
    case "skill":
        return d.loadSkillProficiency(data)
    case "armor":
        return d.loadArmorProficiency(data)
    default:
        return nil, fmt.Errorf("unknown proficiency: %s", ref.Value)
    }
}

func (d *DnD5eLoader) loadWeaponProficiency(data json.RawMessage) (proficiency.Proficiency, error) {
    var profData struct {
        Ref    string `json:"ref"`
        Source string `json:"source"`
        Config struct {
            Weapon string `json:"weapon"`
        } `json:"config"`
    }
    
    if err := json.Unmarshal(data, &profData); err != nil {
        return nil, err
    }
    
    // Create weapon proficiency that will listen for attack events
    wp := &WeaponProficiency{
        SimpleProficiency: proficiency.NewSimpleProficiency(proficiency.SimpleProficiencyConfig{
            ID:      fmt.Sprintf("prof_weapon_%s", profData.Config.Weapon),
            Type:    "proficiency.weapon",
            Subject: profData.Config.Weapon,
            Source:  profData.Source,
        }),
        weapon: profData.Config.Weapon,
    }
    
    return wp, nil
}
```

## Runtime Activation

```go
type Character struct {
    // ... fields ...
    
    features      []features.Feature
    proficiencies []proficiency.Proficiency
    conditions    []conditions.Condition
}

// Activate wires all character components to the event bus
func (c *Character) Activate(bus events.EventBus) error {
    // Apply all features
    for _, f := range c.features {
        if err := f.Apply(bus); err != nil {
            return fmt.Errorf("apply feature %s: %w", f.GetID(), err)
        }
    }
    
    // Apply all proficiencies
    for _, p := range c.proficiencies {
        if err := p.Apply(bus); err != nil {
            return fmt.Errorf("apply proficiency %s: %w", p.GetID(), err)
        }
    }
    
    // Apply active conditions
    for _, cond := range c.conditions {
        if err := cond.Apply(bus); err != nil {
            return fmt.Errorf("apply condition %s: %w", cond.GetID(), err)
        }
    }
    
    return nil
}

// Deactivate removes all event subscriptions
func (c *Character) Deactivate(bus events.EventBus) error {
    for _, f := range c.features {
        if err := f.Remove(bus); err != nil {
            return fmt.Errorf("remove feature %s: %w", f.GetID(), err)
        }
    }
    
    for _, p := range c.proficiencies {
        if err := p.Remove(bus); err != nil {
            return fmt.Errorf("remove proficiency %s: %w", p.GetID(), err)
        }
    }
    
    for _, cond := range c.conditions {
        if err := cond.Remove(bus); err != nil {
            return fmt.Errorf("remove condition %s: %w", cond.GetID(), err)
        }
    }
    
    return nil
}
```

## Complete Loading Example

```go
func main() {
    // Setup registry with module loaders
    registry := &LoaderRegistry{
        modules: make(map[string]ModuleLoader),
    }
    registry.Register("dnd5e", &DnD5eLoader{})
    registry.Register("homebrew", &HomebrewLoader{})
    
    // Create event bus
    eventBus := events.NewBus()
    
    // Create character loader
    loader := &CharacterLoader{
        registry: registry,
        eventBus: eventBus,
    }
    
    // Load character from JSON
    characterJSON, _ := os.ReadFile("ragnar.json")
    character, err := loader.Load(characterJSON)
    if err != nil {
        log.Fatal(err)
    }
    
    // Activate character (wire to event bus)
    if err := character.Activate(eventBus); err != nil {
        log.Fatal(err)
    }
    
    // Character is now ready for runtime
    // - Features subscribed to events
    // - Proficiencies ready to modify rolls
    // - Conditions applying their effects
    
    // Example: Make an attack
    attackEvent := &AttackEvent{
        Attacker: character,
        Weapon:   "longsword",
        Target:   goblin,
    }
    
    // Proficiencies and features automatically participate
    eventBus.Publish(context.Background(), attackEvent)
}
```

## Error Handling and Validation

### Load-Time Validation
- Verify refs are valid format
- Check required fields in JSON
- Ensure state/config match expected schema

### Missing Module Handling
```go
func (cl *CharacterLoader) loadFeature(data FeatureData) (features.Feature, error) {
    ref, _ := core.ParseRef(data.Ref)
    
    loader, err := cl.registry.GetLoader(ref)
    if err != nil {
        // Could return a placeholder/unknown feature
        log.Printf("WARNING: No loader for %s, creating placeholder", data.Ref)
        return &UnknownFeature{
            ref:  ref,
            data: data,
        }, nil
    }
    
    return loader.LoadFeature(ref, data)
}
```

### State Serialization
```go
// Features can export their state for persistence
type StatefulFeature interface {
    features.Feature
    GetState() json.RawMessage
}

// When saving character
func (c *Character) Export() CharacterData {
    data := CharacterData{
        // ... basic fields ...
    }
    
    for _, f := range c.features {
        featData := FeatureData{
            Ref: f.Ref().String(),
        }
        
        if sf, ok := f.(StatefulFeature); ok {
            featData.State = sf.GetState()
        }
        
        data.Features = append(data.Features, featData)
    }
    
    return data
}
```

## Benefits of This Design

1. **Self-Contained**: No external lookups needed at load time
2. **Extensible**: New modules can be added without changing core
3. **Type-Safe**: Each module handles its own types
4. **Event-Driven**: Runtime behavior through event subscriptions
5. **Testable**: Can test loading without full game context

## Open Questions

1. **Lazy Loading**: Should features load immediately or on first use?
2. **Hot Reload**: Can we reload features while character is active?
3. **Version Migration**: How to handle data from older versions?
4. **Partial Loading**: Can we load just combat-relevant features?
5. **Validation Timing**: Validate on load or on activation?

## Next Steps

1. Implement basic CharacterLoader with single module
2. Create RageFeature that actually works via events
3. Test weapon proficiency modifying attack rolls
4. Verify the complete flow works end-to-end