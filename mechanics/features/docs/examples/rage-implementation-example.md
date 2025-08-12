# Detailed Rage Implementation Example

This shows exactly how Rage would work with our new simplified architecture, following the patterns from our Bless example.

## The Complete Rage Implementation

```go
// rulebooks/dnd5e/classes/barbarian/rage.go
package barbarian

import (
    "context"
    "fmt"
    
    "github.com/KirkDiggler/rpg-toolkit/core"
    "github.com/KirkDiggler/rpg-toolkit/dice"
    "github.com/KirkDiggler/rpg-toolkit/events"
    "github.com/KirkDiggler/rpg-toolkit/mechanics/features"
    "github.com/KirkDiggler/rpg-toolkit/mechanics/resources"
)

// RageRef is the identifier for the Rage feature
var RageRef = core.MustNewRef(core.RefInput{
    Module: "dnd5e",
    Type:   "feature",
    Value:  "rage",
})

// BarbarianClass source
var Class = &core.Source{
    Category: core.SourceClass,
    Name:     "barbarian",
}

// RageFeature extends SimpleFeature with rage-specific state
type RageFeature struct {
    *features.SimpleFeature
    usesRemaining int
    maxUses       int
    isActive      bool
    turnsActive   int
}

// NewRage creates a new Rage feature for a barbarian
func NewRage(level int) *RageFeature {
    rage := &RageFeature{
        maxUses:       calculateRageUses(level),
        usesRemaining: calculateRageUses(level),
    }
    
    rage.SimpleFeature = features.NewSimple(
        features.WithRef(RageRef),
        features.FromSource(Class),
        features.AtLevel(1),
        features.OnApply(rage.apply),
        features.OnRemove(rage.remove),
    )
    
    return rage
}

// RageData represents the saved state of a rage feature
type RageData struct {
    Ref           string `json:"ref"`
    Level         int    `json:"level"`
    UsesRemaining int    `json:"uses_remaining"`
    MaxUses       int    `json:"max_uses"`
    IsActive      bool   `json:"is_active"`
    TurnsActive   int    `json:"turns_active"`
}

// LoadRageFromJSON recreates Rage from saved state
func LoadRageFromJSON(data json.RawMessage) (*RageFeature, error) {
    var rageData RageData
    if err := json.Unmarshal(data, &rageData); err != nil {
        return nil, err
    }
    
    rage := &RageFeature{
        usesRemaining: rageData.UsesRemaining,
        maxUses:       rageData.MaxUses,
        isActive:      rageData.IsActive,
        turnsActive:   rageData.TurnsActive,
    }
    
    rage.SimpleFeature = features.NewSimple(
        features.WithRef(RageRef),
        features.FromSource(Class),
        features.AtLevel(rageData.Level),
        features.OnApply(rage.apply),
        features.OnRemove(rage.remove),
    )
    
    return rage, nil
}

// apply sets up all the event subscriptions for rage
func (r *RageFeature) apply(f *features.SimpleFeature, bus events.EventBus) error {
    // Subscribe to feature activation (when player chooses to rage)
    f.Subscribe(bus, "feature.activate", 100, func(ctx context.Context, e events.Event) error {
        featureRef, _ := e.Context().Get("feature_ref")
        if ref, ok := featureRef.(*core.Ref); ok && ref.Equals(RageRef) {
            return r.startRage(ctx, e, bus)
        }
        return nil
    })
    
    // Add damage resistance while raging
    f.Subscribe(bus, events.EventBeforeTakeDamage, 50, func(ctx context.Context, e events.Event) error {
        if !r.isActive {
            return nil
        }
        
        // Check if this is our raging barbarian taking damage
        if e.Target().GetID() != f.Owner().GetID() {
            return nil
        }
        
        damageType, _ := e.Context().Get("damage_type")
        if dt, ok := damageType.(string); ok {
            // Resistance to physical damage
            if dt == "slashing" || dt == "piercing" || dt == "bludgeoning" {
                damage, _ := e.Context().Get("damage")
                if dmg, ok := damage.(int); ok {
                    reducedDamage := dmg / 2
                    e.Context().Set("damage", reducedDamage)
                    e.Context().AddNote(fmt.Sprintf("Rage Resistance: %d → %d", dmg, reducedDamage))
                }
            }
        }
        
        return nil
    })
    
    // Add rage damage bonus
    f.Subscribe(bus, events.EventOnDamageRoll, 50, func(ctx context.Context, e events.Event) error {
        if !r.isActive {
            return nil
        }
        
        // Check if this is our raging barbarian dealing damage
        if e.Source().GetID() != f.Owner().GetID() {
            return nil
        }
        
        // Check if it's a melee weapon attack with Strength
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
    
    // Track rage duration
    f.Subscribe(bus, "turn.end", 100, func(ctx context.Context, e events.Event) error {
        if !r.isActive {
            return nil
        }
        
        // Check if this is the barbarian's turn ending
        if e.Source().GetID() != f.Owner().GetID() {
            return nil
        }
        
        r.turnsActive++
        
        // Check if rage should end (no attack or damage this turn)
        if !r.maintainedRage(e) {
            return r.endRage(ctx, bus)
        }
        
        // Rage ends after 10 turns (1 minute)
        if r.turnsActive >= 10 {
            return r.endRage(ctx, bus)
        }
        
        return nil
    })
    
    // End rage if knocked unconscious
    f.Subscribe(bus, "condition.applied", 100, func(ctx context.Context, e events.Event) error {
        if !r.isActive {
            return nil
        }
        
        condition, _ := e.Context().Get("condition")
        if cond, ok := condition.(string); ok && cond == "unconscious" {
            if e.Target().GetID() == f.Owner().GetID() {
                return r.endRage(ctx, bus)
            }
        }
        
        return nil
    })
    
    // Handle long rest - restore all rage uses
    f.Subscribe(bus, "rest.long", 100, func(ctx context.Context, e events.Event) error {
        if e.Source().GetID() == f.Owner().GetID() {
            r.usesRemaining = r.maxUses
            e.Context().AddNote(fmt.Sprintf("Rage uses restored: %d/%d", r.usesRemaining, r.maxUses))
        }
        return nil
    })
    
    return nil
}

// remove cleans up when feature is removed
func (r *RageFeature) remove(f *features.SimpleFeature, bus events.EventBus) error {
    if r.isActive {
        return r.endRage(context.Background(), bus)
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
    
    // Notify that rage started
    rageStarted := events.NewGameEvent("rage.started", e.Source(), nil)
    rageStarted.Context().Set("uses_remaining", r.usesRemaining)
    bus.Publish(ctx, rageStarted)
    
    return nil
}

// endRage stops the current rage
func (r *RageFeature) endRage(ctx context.Context, bus events.EventBus) error {
    if !r.isActive {
        return nil
    }
    
    r.isActive = false
    r.turnsActive = 0
    
    // Notify that rage ended
    rageEnded := events.NewGameEvent("rage.ended", r.SimpleFeature.Owner(), nil)
    bus.Publish(ctx, rageEnded)
    
    return nil
}

// maintainedRage checks if barbarian attacked or took damage this turn
func (r *RageFeature) maintainedRage(e events.Event) bool {
    // Check event context for attack or damage flags
    attacked, _ := e.Context().Get("made_attack_this_turn")
    tookDamage, _ := e.Context().Get("took_damage_this_turn")
    
    return attacked == true || tookDamage == true
}

// getRageDamageBonus returns the rage damage bonus by level
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

// calculateRageUses returns max rage uses by level
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

// ToData saves the rage state
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

// GetResources returns rage as a consumable resource
func (r *RageFeature) GetResources() []resources.Resource {
    return []resources.Resource{
        resources.NewSimple(
            resources.WithRef(core.MustNewRef(core.RefInput{
                Module: "dnd5e",
                Type:   "resource",
                Value:  "rage_uses",
            })),
            resources.WithCurrent(r.usesRemaining),
            resources.WithMax(r.maxUses),
        ),
    }
}
```

## Usage in Game

```go
// Character creation
barbarian := NewCharacter("Ragnar", "barbarian", 5)
barbarian.AddFeature(barbarian.NewRage(5))

// During combat - player chooses to rage
activateEvent := events.NewGameEvent("feature.activate", barbarian, nil)
activateEvent.Context().Set("feature_ref", barbarian.RageRef)
bus.Publish(ctx, activateEvent)

// Now when barbarian takes damage, it's automatically halved
// When barbarian attacks with strength, rage damage is added
// All handled by the event subscriptions!

// Saving the character
rageData := rage.ToData()  // Returns RageData struct
SaveToDatabase(rageData)

// Loading the character
var savedJSON json.RawMessage
LoadFromDatabase(&savedJSON)
rage, err := LoadRageFromJSON(savedJSON)  // Returns (*RageFeature, error)
if err != nil {
    return err
}
barbarian.AddFeature(rage)
```

## Key Patterns Demonstrated

1. **State Management**: Rage tracks uses, active state, duration
2. **Event-Driven**: Everything works through event subscriptions
3. **Resource Integration**: Rage uses are exposed as a resource
4. **Data Persistence**: Can save/load rage state
5. **Level Scaling**: Damage bonus and uses scale with level
6. **Complex Conditions**: Ends on unconscious, after timeout, or if not maintained

## How This Verifies Our Assumptions

✅ **Simple to implement complex behavior** - All the rage logic fits in one file
✅ **Event bus handles interactions** - No need to check "is raging?" everywhere
✅ **Features are self-contained** - Rage knows everything about rage
✅ **State can be saved/loaded** - Perfect for persistent games
✅ **Extensible** - Subclasses could extend this (Berserker's Frenzy)

## Comparison to Current Architecture

### Before (14 methods to implement):
```go
type RageFeature struct {
    // Must implement ALL of these:
    Key() string
    Name() string
    Description() string
    Type() FeatureType
    Level() int
    Source() string
    IsPassive() bool
    GetTiming() FeatureTiming
    GetModifiers() []events.Modifier
    GetProficiencies() []string
    GetResources() []resources.Resource
    GetEventListeners() []EventListener
    CanTrigger(event events.Event) bool
    TriggerFeature(entity core.Entity, event events.Event) error
    HasPrerequisites() bool
    MeetsPrerequisites(entity core.Entity) bool
    GetPrerequisites() []string
    IsActive(entity core.Entity) bool
    Activate(entity core.Entity, bus events.EventBus) error
    Deactivate(entity core.Entity, bus events.EventBus) error
}
```

### After (just what we need):
```go
type RageFeature struct {
    *features.SimpleFeature  // Handles Apply/Remove
    // Just our state:
    usesRemaining int
    isActive bool
}
```

The complexity is in the BEHAVIOR (the event handlers), not the INTERFACE!