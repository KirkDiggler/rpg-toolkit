# Character Loading Design - Final Version

## Overview

A complete design showing how characters load from data with proper type safety using constants throughout. The game server passes opaque data to the rulebook, which interprets it into working features and proficiencies.

## Architecture Layers

1. **Toolkit** (`rpg-toolkit/`) - Provides infrastructure (EventType, EventBus, etc.)
2. **Rulebook** (`rulebooks/dnd5e/`) - Implements D&D 5e rules using toolkit
3. **Game Server** - Orchestrates, stores data, doesn't understand rules

## Toolkit Types (Already Exist)

```go
// github.com/KirkDiggler/rpg-toolkit/events/types.go
package events

type EventType string  // Toolkit provides the type

type EventBus interface {
    Subscribe(eventType EventType, handler Handler) error
    Publish(eventType EventType, event Event) error
}

// github.com/KirkDiggler/rpg-toolkit/core/ref.go
package core

type Ref struct {
    Module string  // "dnd5e"
    Type   string  // "feature"
    Value  string  // "rage"
}
```

## Rulebook Constants

```go
// rulebooks/dnd5e/constants/events.go
package constants

import "github.com/KirkDiggler/rpg-toolkit/events"

// Combat Events
const (
    AttackRoll    events.EventType = "combat.attack.roll"
    DamageRoll    events.EventType = "combat.damage.roll"
    DamageTaken   events.EventType = "combat.damage.taken"
    CombatStarted events.EventType = "combat.started"
    CombatEnded   events.EventType = "combat.ended"
    TurnStarted   events.EventType = "combat.turn.started"
    TurnEnded     events.EventType = "combat.turn.ended"
)

// Feature Events
const (
    FeatureActivated   events.EventType = "feature.activated"
    FeatureDeactivated events.EventType = "feature.deactivated"
    ResourceConsumed   events.EventType = "resource.consumed"
)

// rulebooks/dnd5e/constants/resources.go
package constants

type ResourceKey string

const (
    RageUses        ResourceKey = "rage_uses"
    SecondWindUses  ResourceKey = "second_wind_uses"
    ActionSurgeUses ResourceKey = "action_surge_uses"
    HitDice         ResourceKey = "hit_dice"
    SpellSlot1      ResourceKey = "spell_slot_1"
    SpellSlot2      ResourceKey = "spell_slot_2"
)

// rulebooks/dnd5e/constants/modifiers.go
package constants

type ModifierSource string

const (
    SourceRage         ModifierSource = "rage"
    SourceProficiency  ModifierSource = "proficiency"
    SourceExpertise    ModifierSource = "expertise"
    SourceBless        ModifierSource = "bless"
    SourceGuidance     ModifierSource = "guidance"
)

// rulebooks/dnd5e/constants/damage.go
package constants

type DamageType string

const (
    Bludgeoning DamageType = "bludgeoning"
    Piercing    DamageType = "piercing"
    Slashing    DamageType = "slashing"
    Fire        DamageType = "fire"
    Cold        DamageType = "cold"
    Lightning   DamageType = "lightning"
    Acid        DamageType = "acid"
    Poison      DamageType = "poison"
    Necrotic    DamageType = "necrotic"
    Radiant     DamageType = "radiant"
    Thunder     DamageType = "thunder"
    Psychic     DamageType = "psychic"
    Force       DamageType = "force"
)

type AttackType string

const (
    MeleeWeapon  AttackType = "melee_weapon"
    RangedWeapon AttackType = "ranged_weapon"
    MeleeSpell   AttackType = "melee_spell"
    RangedSpell  AttackType = "ranged_spell"
)
```

## Character Data Structure

```go
// Data from game server - it doesn't know what this means
type CharacterData struct {
    ID       string                 `json:"id"`
    Name     string                 `json:"name"`
    Level    int                    `json:"level"`
    
    // Core stats
    Abilities    map[string]int     `json:"abilities"`
    Proficiency  int                `json:"proficiency"`
    MaxHP        int                `json:"max_hp"`
    CurrentHP    int                `json:"current_hp"`
    
    // Features and proficiencies as JSON
    Features      []json.RawMessage `json:"features"`
    Proficiencies []json.RawMessage `json:"proficiencies"`
    
    // Resources
    Resources map[string]int        `json:"resources"`
}
```

## The Rulebook

```go
// rulebooks/dnd5e/rulebook.go
package dnd5e

import (
    "github.com/KirkDiggler/rpg-toolkit/events"
    "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/constants"
    "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/features/rage"
    "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/features/secondwind"
)

type Rulebook struct {
    eventBus *events.Bus
}

func NewRulebook(bus *events.Bus) *Rulebook {
    return &Rulebook{
        eventBus: bus,
    }
}

func (rb *Rulebook) LoadCharacter(data CharacterData) (*Character, error) {
    char := &Character{
        ID:          data.ID,
        Name:        data.Name,
        Level:       data.Level,
        Abilities:   data.Abilities,
        Proficiency: data.Proficiency,
        MaxHP:       data.MaxHP,
        CurrentHP:   data.CurrentHP,
        
        // Convert string resources to typed
        resources:     make(map[constants.ResourceKey]int),
        features:      []Feature{},
        proficiencies: []Proficiency{},
    }
    
    // Convert resources to typed constants
    for key, value := range data.Resources {
        char.resources[constants.ResourceKey(key)] = value
    }
    
    // Load features
    for _, featJSON := range data.Features {
        feature, err := rb.loadFeature(char, featJSON)
        if err != nil {
            // Log and skip unknown features
            continue
        }
        char.features = append(char.features, feature)
    }
    
    // Load proficiencies
    for _, profJSON := range data.Proficiencies {
        prof, err := rb.loadProficiency(char, profJSON)
        if err != nil {
            continue
        }
        char.proficiencies = append(char.proficiencies, prof)
    }
    
    return char, nil
}

func (rb *Rulebook) loadFeature(owner *Character, data json.RawMessage) (Feature, error) {
    // Peek at the type
    var peek struct {
        Type string `json:"type"`
    }
    json.Unmarshal(data, &peek)
    
    switch peek.Type {
    case "rage":
        return rage.Load(owner, data)
    case "second_wind":
        return secondwind.Load(owner, data)
    default:
        return nil, fmt.Errorf("unknown feature: %s", peek.Type)
    }
}

func (rb *Rulebook) loadProficiency(owner *Character, data json.RawMessage) (Proficiency, error) {
    var peek struct {
        Type   string `json:"type"`   // "weapon", "skill"
        Target string `json:"target"` // "longsword", "athletics"
    }
    json.Unmarshal(data, &peek)
    
    switch peek.Type {
    case "weapon":
        return NewWeaponProficiency(owner, peek.Target), nil
    case "skill":
        return NewSkillProficiency(owner, peek.Target), nil
    default:
        return nil, fmt.Errorf("unknown proficiency type: %s", peek.Type)
    }
}
```

## Rage Feature Implementation

```go
// rulebooks/dnd5e/features/rage/rage.go
package rage

import (
    "github.com/KirkDiggler/rpg-toolkit/events"
    "github.com/KirkDiggler/rpg-toolkit/mechanics/features"
    "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/constants"
)

type RageFeature struct {
    *features.SimpleFeature
    
    owner         *Character
    level         int
    damageBonus   int
    usesRemaining int
    maxUses       int
    isActive      bool
}

// Load creates a rage feature from JSON data
func Load(owner *Character, data json.RawMessage) (*RageFeature, error) {
    // For simplicity, just create with defaults
    // Real implementation would parse data
    
    rage := &RageFeature{
        owner:       owner,
        level:       owner.Level,
        damageBonus: calculateRageDamage(owner.Level),
        maxUses:     calculateMaxRages(owner.Level),
    }
    
    // Check current uses from resources
    if uses, ok := owner.resources[constants.RageUses]; ok {
        rage.usesRemaining = uses
    } else {
        rage.usesRemaining = rage.maxUses
    }
    
    // Create the base feature
    rage.SimpleFeature = features.NewSimpleFeature(features.SimpleFeatureConfig{
        Ref:  core.NewRef("dnd5e", "feature", "rage"),
        Name: "Rage",
        Description: "In battle, you fight with primal ferocity",
        OnActivate: rage.activate,
    })
    
    return rage, nil
}

// activate is called when the player uses the rage action
func (r *RageFeature) activate(owner core.Entity, ctx *features.ActivateContext) error {
    // Check if we have uses
    if r.usesRemaining <= 0 {
        return ErrNoRageUses
    }
    
    // Check if already raging
    if r.isActive {
        return ErrAlreadyRaging
    }
    
    // Consume a use
    r.usesRemaining--
    r.owner.UpdateResource(constants.RageUses, r.usesRemaining)
    
    // Mark as active
    r.isActive = true
    
    // Return nil - the Apply method will handle event subscriptions
    return nil
}

// Apply wires rage to the event bus when activated
func (r *RageFeature) Apply(bus events.EventBus) error {
    if !r.isActive {
        return nil
    }
    
    // Subscribe to damage rolls to add rage bonus
    bus.Subscribe(constants.DamageRoll, r.handleDamageBonus)
    
    // Subscribe to incoming damage for resistance
    bus.Subscribe(constants.DamageTaken, r.handleDamageResistance)
    
    // Subscribe to turn end to check if rage continues
    bus.Subscribe(constants.TurnEnded, r.handleTurnEnd)
    
    // Publish rage started event
    bus.Publish(constants.FeatureActivated, &FeatureEvent{
        Source:  r.owner,
        Feature: "rage",
    })
    
    return nil
}

func (r *RageFeature) handleDamageBonus(e events.Event) error {
    // Only apply to our attacks
    if e.Source() != r.owner {
        return nil
    }
    
    // Only melee weapon attacks
    attackType := e.Context().GetString("attack_type")
    if attackType != string(constants.MeleeWeapon) {
        return nil
    }
    
    // Add rage damage bonus
    e.Context().AddModifier(events.NewModifier(
        string(constants.SourceRage),
        events.ModifierDamageBonus,
        r.damageBonus,
    ))
    
    return nil
}

func (r *RageFeature) handleDamageResistance(e events.Event) error {
    // Only apply to damage against us
    if e.Target() != r.owner {
        return nil
    }
    
    // Check damage type
    damageType := constants.DamageType(e.Context().GetString("damage_type"))
    
    // Resistance to physical damage
    if damageType == constants.Bludgeoning || 
       damageType == constants.Piercing || 
       damageType == constants.Slashing {
        
        damage := e.Context().GetInt("damage")
        e.Context().Set("damage", damage/2)
        e.Context().Set("resistance_applied", true)
    }
    
    return nil
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
        return -1 // Unlimited
    }
}
```

## Weapon Proficiency Implementation

```go
// rulebooks/dnd5e/proficiency/weapon.go
package proficiency

import (
    "github.com/KirkDiggler/rpg-toolkit/events"
    "github.com/KirkDiggler/rpg-toolkit/mechanics/proficiency"
    "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/constants"
)

type WeaponProficiency struct {
    *proficiency.SimpleProficiency
    
    owner  *Character
    weapon string
}

func NewWeaponProficiency(owner *Character, weapon string) *WeaponProficiency {
    wp := &WeaponProficiency{
        owner:  owner,
        weapon: weapon,
    }
    
    wp.SimpleProficiency = proficiency.NewSimpleProficiency(proficiency.SimpleProficiencyConfig{
        ID:      fmt.Sprintf("prof_weapon_%s_%s", owner.ID, weapon),
        Type:    "proficiency.weapon",
        Subject: weapon,
        Source:  "class", // Could be more specific
        ApplyFunc: wp.apply,
    })
    
    return wp
}

func (wp *WeaponProficiency) apply(bus events.EventBus) error {
    // Subscribe to attack rolls
    return bus.Subscribe(constants.AttackRoll, wp.handleAttackRoll)
}

func (wp *WeaponProficiency) handleAttackRoll(e events.Event) error {
    // Only apply to our owner
    if e.Source() != wp.owner {
        return nil
    }
    
    // Check if using our weapon
    weapon := e.Context().GetString("weapon")
    if weapon != wp.weapon {
        return nil
    }
    
    // Add proficiency bonus
    e.Context().AddModifier(events.NewModifier(
        string(constants.SourceProficiency),
        events.ModifierAttackBonus,
        wp.owner.Proficiency,
    ))
    
    return nil
}
```

## Character Activation

```go
// rulebooks/dnd5e/character.go
package dnd5e

type Character struct {
    ID          string
    Name        string
    Level       int
    Proficiency int
    
    resources     map[constants.ResourceKey]int
    features      []Feature
    proficiencies []Proficiency
}

// Activate wires the character to the event bus
func (c *Character) Activate(bus events.EventBus) error {
    // Apply all proficiencies (always active)
    for _, prof := range c.proficiencies {
        if err := prof.Apply(bus); err != nil {
            return fmt.Errorf("apply proficiency: %w", err)
        }
    }
    
    // Features aren't applied until activated by player action
    return nil
}

// ActivateFeature activates a specific feature (player action)
func (c *Character) ActivateFeature(featureRef string, bus events.EventBus) error {
    for _, feature := range c.features {
        if feature.Ref().String() == featureRef {
            // Activate the feature
            if err := feature.Activate(c); err != nil {
                return err
            }
            
            // Apply to event bus
            return feature.Apply(bus)
        }
    }
    
    return fmt.Errorf("feature not found: %s", featureRef)
}

// UpdateResource updates a resource value
func (c *Character) UpdateResource(key constants.ResourceKey, value int) {
    c.resources[key] = value
}
```

## Complete Flow Example

```go
func main() {
    // 1. Game server has character data (from DB)
    charData := CharacterData{
        ID:          "char_123",
        Name:        "Ragnar",
        Level:       5,
        Proficiency: 3,
        Abilities: map[string]int{
            "strength": 18,
            "dexterity": 14,
        },
        MaxHP:     55,
        CurrentHP: 45,
        
        Features: []json.RawMessage{
            json.RawMessage(`{"type": "rage"}`),
            json.RawMessage(`{"type": "second_wind"}`),
        },
        
        Proficiencies: []json.RawMessage{
            json.RawMessage(`{"type": "weapon", "target": "greatsword"}`),
            json.RawMessage(`{"type": "skill", "target": "athletics"}`),
        },
        
        Resources: map[string]int{
            "rage_uses": 3,
            "second_wind_uses": 1,
        },
    }
    
    // 2. Create event bus
    eventBus := events.NewBus()
    
    // 3. Create rulebook
    rulebook := dnd5e.NewRulebook(eventBus)
    
    // 4. Load character through rulebook
    character, err := rulebook.LoadCharacter(charData)
    if err != nil {
        log.Fatal(err)
    }
    
    // 5. Activate character (wires proficiencies)
    character.Activate(eventBus)
    
    // 6. Combat starts
    eventBus.Publish(constants.CombatStarted, &CombatEvent{})
    
    // 7. Player's turn - activate rage
    err = character.ActivateFeature("dnd5e:feature:rage", eventBus)
    // Rage is now listening to events
    
    // 8. Player attacks with greatsword
    attackEvent := &AttackEvent{
        Source:     character,
        Target:     goblin,
        Weapon:     "greatsword",
        AttackType: string(constants.MeleeWeapon),
    }
    
    eventBus.Publish(constants.AttackRoll, attackEvent)
    // Weapon proficiency adds +3 to attack
    
    // 9. Roll damage
    damageEvent := &DamageEvent{
        Source:     character,
        Target:     goblin,
        AttackType: string(constants.MeleeWeapon),
    }
    
    eventBus.Publish(constants.DamageRoll, damageEvent)
    // Rage adds +2 to damage
    
    // 10. Goblin attacks back
    incomingDamage := &DamageEvent{
        Source:     goblin,
        Target:     character,
        Damage:     10,
        DamageType: string(constants.Slashing),
    }
    
    eventBus.Publish(constants.DamageTaken, incomingDamage)
    // Rage reduces damage to 5 (resistance)
}
```

## Key Design Points

1. **Constants everywhere** - No magic strings, all typed constants
2. **Toolkit provides types** - EventType, EventBus, etc.
3. **Rulebook defines values** - Event names, resource keys, etc.
4. **Data-driven loading** - Features and proficiencies from JSON
5. **Event-driven runtime** - Everything communicates via events
6. **Clean separation** - Game server doesn't understand rules

## Benefits

- **Type safety** - Can't typo event names or resource keys
- **Discoverable** - IDE knows all constants
- **Extensible** - Easy to add new features/proficiencies
- **Testable** - Mock event bus, test in isolation
- **Data-driven** - Characters fully defined by data