# Definitions - What Everything Actually Is

## Game Domain Definitions (D&D 5e)

### Features
**What it is**: A class/race ability that defines your character
- Examples: Rage, Sneak Attack, Divine Sense, Wild Shape
- Granted by: Class, race, or feat
- Identity: "I'm a barbarian, I have Rage"

### Spells
**What it is**: Magical effects you can cast
- Examples: Fireball, Cure Wounds, Shield
- Granted by: Class spell list, learned/prepared
- Identity: "I'm a wizard, I know Fireball"

### Conditions
**What it is**: Status effects affecting a creature
- Examples: Poisoned, Stunned, Blessed, Raging
- Granted by: Spells, features, environment, items
- Identity: "I am poisoned"

### Actions
**What it is**: Things you can DO on your turn
- Examples: Attack, Cast a Spell, Dash, Activate Feature
- Granted by: Game rules + your features/spells
- Identity: "I take the Attack action"

### Effects
**What it is**: The mechanical changes to game state
- Examples: +2 damage, resistance to fire, advantage on saves
- Granted by: Everything above
- Identity: Technical implementation detail

## The Layered Architecture

```go
// D&D 5e domain layer - what players see
rulebooks/dnd5e/
├── features/     // Character features
├── spells/       // Spell implementations  
├── conditions/   // Status conditions
└── actions/      // Available actions

// Toolkit mechanics layer - how it works
mechanics/
├── effects/      // Core effect system (all use this)
├── actions/      // Action framework
└── resources/    // Resource management
```

## D&D 5e Character Has First-Class Features

```go
// rulebooks/dnd5e/character.go
type Character struct {
    // First-class game concepts
    Features   []Feature    // My class/race abilities
    Spells     []Spell      // My known/prepared spells
    Conditions []Condition  // My current status effects
    
    // Under the hood
    actions    map[string]Action  // What I can actually DO
    effects    []Effect           // Current mechanical effects
}

// Features are first-class
func (c *Character) LoadFeaturesFromJSON(data []json.RawMessage) error {
    for _, raw := range data {
        feature := dnd5e.LoadFeatureFromJSON(raw)
        c.Features = append(c.Features, feature)
        
        // Feature might provide actions
        if actions := feature.GetActions(); len(actions) > 0 {
            for _, action := range actions {
                c.actions[action.Ref()] = action
            }
        }
    }
}
```

## Features Can Provide Actions

```go
// rulebooks/dnd5e/features/rage.go
type RageFeature struct {
    *effects.Core  // Powered by effects under the hood
    level         int
    usesRemaining int
}

// Feature is a first-class concept
func (r *RageFeature) Name() string { return "Rage" }
func (r *RageFeature) Description() string { 
    return "In battle, you fight with primal ferocity..."
}

// But it PROVIDES an action
func (r *RageFeature) GetActions() []Action {
    return []Action{
        &ActivateRageAction{
            feature: r,
            cost:    Cost{Type: "rage_use", Amount: 1},
        },
    }
}

// The action is what you DO
type ActivateRageAction struct {
    feature *RageFeature
    cost    Cost
}

func (a *ActivateRageAction) Activate(target Entity) error {
    if a.feature.usesRemaining <= 0 {
        return ErrNoUses
    }
    
    a.feature.usesRemaining--
    
    // Apply the rage condition
    rageCondition := conditions.NewRaging(a.feature.owner)
    a.feature.owner.AddCondition(rageCondition)
    
    return nil
}
```

## Spells Are First-Class Too

```go
// rulebooks/dnd5e/spells/fireball.go
type FireballSpell struct {
    *effects.Core
    level int
}

func (f *FireballSpell) Name() string { return "Fireball" }
func (f *FireballSpell) Level() int { return 3 }
func (f *FireballSpell) School() string { return "Evocation" }

// Spell provides a Cast action
func (f *FireballSpell) GetAction() Action {
    return &CastFireballAction{
        spell: f,
        cost:  Cost{Type: "spell_slot", Level: 3},
    }
}
```

## The Beautiful Separation

```go
// What the game knows about (D&D 5e layer)
type Feature interface {
    Name() string
    Description() string
    Source() string  // "Barbarian class"
    GetActions() []Action  // What actions does this provide?
}

type Spell interface {
    Name() string
    Level() int
    School() string
    GetAction() Action  // Cast spell action
}

type Condition interface {
    Name() string
    Description() string
    GetEffects() []Effect  // What effects does this apply?
}

// How it actually works (mechanics layer)
type Action interface {
    CanActivate() bool
    Activate(target Entity) error
    GetCost() Cost
}

type Effect interface {
    Apply(bus EventBus) error
    Remove(bus EventBus) error
}
```

## Loading Preserves Domain Concepts

```go
// Loading a character preserves the domain model
func LoadCharacterFromJSON(data CharacterData) *Character {
    char := &Character{
        Features:   []Feature{},
        Spells:     []Spell{},
        Conditions: []Condition{},
    }
    
    // Load features as features
    for _, raw := range data.Features {
        feature := dnd5e.LoadFeatureFromJSON(raw)
        char.Features = append(char.Features, feature)
    }
    
    // Load spells as spells
    for _, raw := range data.Spells {
        spell := dnd5e.LoadSpellFromJSON(raw)
        char.Spells = append(char.Spells, spell)
    }
    
    // But under the hood, collect all actions
    char.rebuildActionList()
    
    return char
}

func (c *Character) rebuildActionList() {
    c.actions = make(map[string]Action)
    
    // Collect actions from features
    for _, feat := range c.Features {
        for _, action := range feat.GetActions() {
            c.actions[action.Ref()] = action
        }
    }
    
    // Collect actions from spells
    for _, spell := range c.Spells {
        c.actions[spell.GetAction().Ref()] = spell.GetAction()
    }
    
    // Add basic actions
    c.actions["attack"] = &AttackAction{owner: c}
    c.actions["dash"] = &DashAction{owner: c}
}
```

## The Key Insight

**D&D 5e concepts remain first-class in the D&D 5e layer!**

- Characters HAVE features (Rage)
- Characters KNOW spells (Fireball)  
- Characters SUFFER conditions (Poisoned)

But under the hood:
- Features/spells provide ACTIONS (what you can do)
- Actions consume RESOURCES and create EFFECTS
- Effects modify game state through EVENTS

This preserves the game's domain model while using consistent mechanics underneath!