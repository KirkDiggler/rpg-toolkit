# Feature Activation Pattern

## The Missing Interface Methods

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
    
    // MISSING: What players need to see!
    Name() string                    // "Rage"
    Description() string             // "Enter a battle fury..."
    CanActivate() bool              // Do I have uses left?
    GetActivationCost() string      // "1 use" or "1 ki point"
    GetRemainingUses() string       // "2/3 uses"
    Activate() error                // Player clicked the button!
    IsActive() bool                 // Currently active?
}
```

## How Players See Features

```go
// API endpoint for UI
func GetCharacterActions(char *Character) []ActionInfo {
    var actions []ActionInfo
    
    for _, feature := range char.features {
        if feature.CanActivate() {
            actions = append(actions, ActionInfo{
                Ref:         feature.Ref().String(),
                Name:        feature.Name(),
                Description: feature.Description(),
                Cost:        feature.GetActivationCost(),
                Remaining:   feature.GetRemainingUses(),
                IsActive:    feature.IsActive(),
            })
        }
    }
    
    return actions
}

// Returns something like:
[
    {
        "ref": "dnd5e:feature:rage",
        "name": "Rage",
        "description": "Enter a fury that grants resistance...",
        "cost": "1 use",
        "remaining": "2/3 uses",
        "is_active": false
    },
    {
        "ref": "dnd5e:feature:second_wind",
        "name": "Second Wind",
        "description": "Regain hit points equal to...",
        "cost": "1 use",
        "remaining": "1/1 use",
        "is_active": false
    }
]
```

## Player Activates Feature

```go
// Player clicked "Rage" button
func ActivateFeature(char *Character, featureRef string) error {
    for _, feature := range char.features {
        if feature.Ref().String() == featureRef {
            return feature.Activate()
        }
    }
    return fmt.Errorf("feature not found: %s", featureRef)
}

// Rage implements Activate
func (r *RageFeature) Activate() error {
    if r.isActive {
        return fmt.Errorf("already raging")
    }
    
    if r.usesRemaining <= 0 {
        return fmt.Errorf("no rage uses remaining")
    }
    
    // Fire the activation event
    activateEvent := events.NewGameEvent("feature.activate", r.owner, nil)
    activateEvent.Context().Set("feature_ref", RageRef)
    
    // The Apply() subscriptions will handle the actual activation
    return r.eventBus.Publish(context.Background(), activateEvent)
}

func (r *RageFeature) CanActivate() bool {
    return !r.isActive && r.usesRemaining > 0
}

func (r *RageFeature) GetRemainingUses() string {
    if r.maxUses == -1 {
        return "∞"
    }
    return fmt.Sprintf("%d/%d uses", r.usesRemaining, r.maxUses)
}

func (r *RageFeature) Name() string {
    return "Rage"
}

func (r *RageFeature) Description() string {
    damage := r.getRageDamageBonus()
    return fmt.Sprintf("Enter a battle fury. Gain +%d damage and resistance to physical damage.", damage)
}
```

## Different Activation Patterns

```go
// Some features are always active (no Activate method needed)
func (d *Darkvision) CanActivate() bool {
    return false  // Passive feature
}

// Some features are reactions
func (u *UncannyDodge) CanActivate() bool {
    return false  // Only triggers on events, not manually activated
}

// Some features have multiple modes
type ActionSurge struct {
    mode string  // "extra_action" or "extra_attack"
}

func (a *ActionSurge) GetActivationOptions() []string {
    return []string{"Extra Action", "Extra Attack"}
}

func (a *ActionSurge) ActivateWithOption(option string) error {
    a.mode = option
    // ...
}
```

## The Complete UI Flow

```go
// 1. UI requests available actions
GET /api/character/actions
Response: [
    {"ref": "dnd5e:feature:rage", "name": "Rage", "remaining": "2/3"},
    {"ref": "dnd5e:spell:fireball", "name": "Fireball", "remaining": "1 3rd level slot"}
]

// 2. Player clicks "Rage"
POST /api/character/activate
Body: {"ref": "dnd5e:feature:rage"}

// 3. Server activates feature
char.GetFeature("dnd5e:feature:rage").Activate()

// 4. Feature fires event, subscriptions handle the rest
// 5. UI updates to show Rage is active
```

## So The Interface Really Needs

```go
type Feature interface {
    // Core
    Ref() *core.Ref
    Apply(events.EventBus) error
    Remove(events.EventBus) error
    
    // UI Display
    Name() string
    Description() string
    
    // Activation
    CanActivate() bool
    Activate() error
    IsActive() bool
    GetRemainingUses() string
    
    // Persistence
    ToJSON() json.RawMessage
    IsDirty() bool
    MarkClean()
    
    // Progression
    UpdateLevel(int) error
}
```

This gives the UI everything it needs to:
1. Show available features
2. Display remaining uses
3. Let players activate them
4. Show which are active

The game server still doesn't need to know WHAT Rage does - it just calls Activate() and the feature handles itself through events!