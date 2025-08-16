# D&D 5e Features Package

This package implements D&D 5e character features (rage, second wind, action surge, etc.) as Actions.

## Core Architecture

The game server knows:
- It's running D&D 5e (not generic)
- Features use `FeatureInput` 
- How to route refs like `"dnd5e.features.rage"` to this package
- Characters either have an action or they don't

The game server DOESN'T know:
- What rage is
- What features exist
- What each feature does

## Input Contract

All D&D 5e features use the same input structure for consistency:

```go
// rulebooks/dnd5e/types.go
package dnd5e

type FeatureInput struct {
    Target   core.Entity    `json:"target,omitempty"`   // For targeted features
    Choice   string         `json:"choice,omitempty"`   // For features with options
    Data     map[string]any `json:"data,omitempty"`     // Feature-specific data
}
```

Most features ignore most fields, but having a consistent structure means the game server can handle all features the same way.

## D&D 5e Feature Interface

```go
// rulebooks/dnd5e/feature.go
package dnd5e

import (
    "context"
    "github.com/KirkDiggler/rpg-toolkit/core"
)

// Feature is the D&D 5e specific interface for character features
// It extends core.Action but can add D&D 5e specific methods
type Feature interface {
    core.Action[FeatureInput]
    
    // D&D 5e specific methods
    GetResourceType() ResourceType  // rage uses, ki points, etc.
    ResetsOn() ResetType            // short rest, long rest, dawn
    GetPrerequisites() []string     // level requirements, etc.
}
```

## Loading Features

```go
// features/loader.go
package features

import (
    "encoding/json"
    "github.com/KirkDiggler/rpg-toolkit/core"
    "github.com/KirkDiggler/rpg-toolkit/core/events"
    dnd5e "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e"
)

// Feature type constants
const (
    RageKey        = "rage"
    SecondWindKey  = "second_wind"
    ActionSurgeKey = "action_surge"
)

// Event refs for rage events
var (
    RageStartedRef = core.MustParseRef("dnd5e.events.rage_started")
    RageEndedRef   = core.MustParseRef("dnd5e.events.rage_ended")
)

// LoadJSON creates a feature from JSON data
// Returns D&D 5e Feature interface, not generic Action
func LoadJSON(data []byte) (dnd5e.Feature, error) {
    var input struct {
        Ref  string          `json:"ref"`  // "dnd5e.features.rage"
        ID   string          `json:"id"`   // "barbarian-1:rage"
        Data json.RawMessage `json:"data"` // Feature-specific config
    }
    
    if err := json.Unmarshal(data, &input); err != nil {
        return nil, err
    }
    
    // Parse ref to get feature type
    ref, err := core.ParseString(input.Ref)
    if err != nil {
        return nil, err
    }
    
    // ref.Module() = "dnd5e"
    // ref.Type() = "features"  
    // ref.Value() = "rage"
    
    switch ref.Value() {
    case RageKey:
        var rageData struct {
            Uses  int `json:"uses"`  // 3 at level 1
            Level int `json:"level"` // barbarian level
        }
        json.Unmarshal(input.Data, &rageData)
        
        return &Rage{
            id:    input.ID,
            uses:  rageData.Uses,
            level: rageData.Level,
        }, nil
        
    case SecondWindKey:
        // Similar pattern...
        
    default:
        return nil, fmt.Errorf("unknown feature: %s", ref.Value())
    }
}
```

## Example: Rage Implementation

```go
// features/rage.go
package features

// RageStartedEvent is published when rage begins
type RageStartedEvent struct {
    *events.BaseEvent
    Owner       core.Entity
    DamageBonus int
    Duration    int // rounds
}

type Rage struct {
    id    string
    uses  int
    level int
    
    // State tracked locally for now
    // Real implementation would use event bus
    active bool
}

// Implements core.Entity
func (r *Rage) GetID() string   { return r.id }
func (r *Rage) GetType() string { return "feature" }

// Implements dnd5e.Feature specific methods
func (r *Rage) GetResourceType() dnd5e.ResourceType { 
    return dnd5e.ResourceRageUses 
}

func (r *Rage) ResetsOn() dnd5e.ResetType { 
    return dnd5e.ResetLongRest 
}

func (r *Rage) GetPrerequisites() []string {
    return []string{"class:barbarian", "level:1"}
}

// Implements core.Action[FeatureInput]
func (r *Rage) CanActivate(ctx context.Context, owner core.Entity, input FeatureInput) error {
    // Rage doesn't use input, but takes FeatureInput for consistency
    
    if r.uses <= 0 {
        return errors.New("no rage uses remaining")
    }
    
    if r.active {
        return errors.New("already raging")
    }
    
    // Could check other conditions via event bus
    return nil
}

func (r *Rage) Activate(ctx context.Context, owner core.Entity, input FeatureInput) error {
    if err := r.CanActivate(ctx, owner, input); err != nil {
        return err
    }
    
    r.uses--
    r.active = true
    
    // Publish events for effects
    bus := events.FromContext(ctx)
    
    // Damage bonus
    damageBonus := 2
    if r.level >= 16 {
        damageBonus = 4
    } else if r.level >= 9 {
        damageBonus = 3
    }
    
    // Create a rage started event
    rageEvent := &RageStartedEvent{
        BaseEvent: events.NewBaseEvent(RageStartedRef),
        Owner: owner,
        DamageBonus: damageBonus,
        Duration: 10, // rounds
    }
    
    bus.Publish(rageEvent)
    
    // The event system handles applying modifiers, resistances, etc.
    
    return nil
}
```

## Game Server Integration

```go
// The game server handles feature activation generically
func (gs *GameServer) HandleFeatureActivation(
    player core.Entity, 
    featureRef string,
    inputData map[string]any,
) error {
    // Get the character's action
    char := player.(*Character)
    action := char.GetAction(featureRef)
    if action == nil {
        return fmt.Errorf("you don't have %s", featureRef)
    }
    
    // Build standard FeatureInput
    input := dnd5e.FeatureInput{
        Target: gs.ParseTarget(inputData["target"]),
        Choice: inputData["choice"].(string),
        Data:   inputData["data"].(map[string]any),
    }
    
    // Activate - game server doesn't know what happens
    return action.Activate(ctx, player, input)
}

// Player clicks Rage button in UI
// UI sends: {"action": "feature", "ref": "dnd5e.features.rage", "input": {}}
// Game server routes to HandleFeatureActivation
```

## Command Handling

The rulebook provides command parsing since it knows D&D 5e commands:

```go
// rulebooks/dnd5e/commands.go
package dnd5e

func (rb *Rulebook) HandleCommand(ctx context.Context, player core.Entity, cmd string) error {
    switch cmd {
    case "/rage":
        return rb.gameServer.HandleFeatureActivation(
            player, 
            "dnd5e.features.rage",
            map[string]any{},
        )
        
    case "/second_wind":
        return rb.gameServer.HandleFeatureActivation(
            player,
            "dnd5e.features.second_wind", 
            map[string]any{},
        )
        
    default:
        return fmt.Errorf("unknown command: %s", cmd)
    }
}
```

## Key Design Points

1. **Consistent Input Type**: All features use `FeatureInput` even if they ignore fields
2. **No Type Leakage**: Game server never imports feature types, just uses `core.Action[FeatureInput]`
3. **Event-Driven Effects**: Features publish events, don't directly modify state
4. **Rulebook Owns Commands**: The rulebook knows "/rage" means activate rage feature
5. **Simple Validation**: Game server just checks if character has the action

## Testing

```go
func TestRageFeature(t *testing.T) {
    // Create rage from JSON
    featureJSON := `{
        "ref": "dnd5e.features.rage",
        "id": "test-rage",
        "data": {"uses": 3, "level": 5}
    }`
    
    action, err := features.LoadJSON([]byte(featureJSON))
    require.NoError(t, err)
    
    // Create mock character
    barbarian := &MockCharacter{id: "conan"}
    
    // Can activate with uses remaining
    input := dnd5e.FeatureInput{} // rage needs no input
    err = action.CanActivate(ctx, barbarian, input)
    assert.NoError(t, err)
    
    // Activate and verify uses consumed
    err = action.Activate(ctx, barbarian, input)
    assert.NoError(t, err)
    
    // Verify events were published
    // Check damage bonus event
    // Check resistance event
}
```