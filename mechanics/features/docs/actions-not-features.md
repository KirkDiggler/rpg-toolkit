# Actions, Not Features! 

## The Revelation

Features, spells, abilities - they're all just ACTIONS that:
- Can be activated
- Might need targets
- Consume resources
- Apply effects

## The Real Interface

```go
// mechanics/actions/action.go
package actions

type Action interface {
    core.Entity
    Ref() *core.Ref
    
    // Display
    Name() string
    Description() string
    Icon() string  // For UI
    
    // Activation
    CanActivate() bool
    NeedsTarget() bool
    ValidTargets() []TargetType  // Self, Enemy, Ally, Area
    Activate(target core.Entity) error  // Target might be nil for self
    
    // Resources
    GetCost() Cost  // What it consumes
    GetUses() Uses  // How many left
    
    // Effects (what actually happens)
    Apply(bus events.EventBus) error
    Remove(bus events.EventBus) error
    
    // State
    IsActive() bool
    IsDirty() bool
    MarkClean()
    
    // Persistence
    ToJSON() json.RawMessage
}

type Cost struct {
    Type   string  // "rage_use", "spell_slot", "ki_point", "action", "bonus_action"
    Amount int     // How many
    Level  int     // For spell slots
}

type Uses struct {
    Current int
    Max     int
    Refresh string  // "short_rest", "long_rest", "turn", "never"
}
```

## Everything is an Action

```go
// Rage is an action
type RageAction struct {
    *actions.BaseAction
    usesRemaining int
    isActive      bool
}

func (r *RageAction) GetCost() actions.Cost {
    return actions.Cost{
        Type:   "rage_use",
        Amount: 1,
    }
}

func (r *RageAction) Activate(target core.Entity) error {
    // Rage targets self (target is nil or self)
    if r.usesRemaining <= 0 {
        return ErrNoUses
    }
    
    r.usesRemaining--
    r.isActive = true
    
    // Publish activation event
    event := events.NewGameEvent("action.activated", r.owner, r.owner)
    event.Context().Set("action_ref", RageRef)
    return r.bus.Publish(context.Background(), event)
}

// Fireball is an action
type FireballAction struct {
    *actions.BaseAction
    spellLevel int
}

func (f *FireballAction) GetCost() actions.Cost {
    return actions.Cost{
        Type:  "spell_slot",
        Level: 3,  // 3rd level slot minimum
    }
}

func (f *FireballAction) NeedsTarget() bool {
    return true  // Needs a point to target
}

func (f *FireballAction) ValidTargets() []TargetType {
    return []TargetType{actions.TargetPoint}  // Target a location
}

func (f *FireballAction) Activate(target core.Entity) error {
    // Check spell slots
    if !f.owner.HasSpellSlot(3) {
        return ErrNoSpellSlots
    }
    
    f.owner.ConsumeSpellSlot(3)
    
    // Fire the spell event
    event := events.NewGameEvent("spell.cast", f.owner, target)
    event.Context().Set("spell_ref", FireballRef)
    event.Context().Set("damage", "8d6")
    event.Context().Set("save_dc", f.calculateDC())
    event.Context().Set("area", "20ft radius")
    
    return f.bus.Publish(context.Background(), event)
}

// Attack is an action
type AttackAction struct {
    *actions.BaseAction
    weapon *Weapon
}

func (a *AttackAction) GetCost() actions.Cost {
    return actions.Cost{
        Type:   "action",  // Uses your action for the turn
        Amount: 1,
    }
}

func (a *AttackAction) NeedsTarget() bool {
    return true
}

func (a *AttackAction) Activate(target core.Entity) error {
    // Make attack roll
    event := events.NewGameEvent("attack.roll", a.owner, target)
    event.Context().Set("weapon", a.weapon)
    return a.bus.Publish(context.Background(), event)
}
```

## Character Has Actions, Not Features

```go
type Character struct {
    // Actions available to this character
    actions map[string]Action  // Keyed by ref
    
    // Resources that actions consume
    resources map[string]Resource
}

// GetAvailableActions returns what player can do
func (c *Character) GetAvailableActions() []ActionInfo {
    var available []ActionInfo
    
    for _, action := range c.actions {
        if action.CanActivate() {
            available = append(available, ActionInfo{
                Ref:         action.Ref().String(),
                Name:        action.Name(),
                Description: action.Description(),
                Cost:        action.GetCost(),
                Uses:        action.GetUses(),
                NeedsTarget: action.NeedsTarget(),
            })
        }
    }
    
    return available
}

// ActivateAction with optional target
func (c *Character) ActivateAction(ref string, targetID string) error {
    action, exists := c.actions[ref]
    if !exists {
        return ErrActionNotFound
    }
    
    var target core.Entity
    if action.NeedsTarget() {
        if targetID == "" {
            return ErrTargetRequired
        }
        target = c.room.GetEntity(targetID)
        if target == nil {
            return ErrInvalidTarget
        }
    }
    
    return action.Activate(target)
}
```

## The Resource System

```go
// Actions consume resources
type Resource interface {
    Type() string
    Current() int
    Max() int
    CanConsume(amount int) bool
    Consume(amount int) error
    Restore(amount int)
    RefreshOn() string  // "short_rest", "long_rest"
}

// Examples
type RageUses struct {
    current int
    max     int
}

type SpellSlots struct {
    slots map[int]int  // Level -> remaining
}

type KiPoints struct {
    current int
    max     int
}
```

## This Unifies Everything!

- **Rage** = Action that costs rage uses
- **Fireball** = Action that costs spell slots
- **Attack** = Action that costs your action
- **Second Wind** = Action that costs its use
- **Sneak Attack** = Action that's free but once/turn

They all:
- Have costs (resources)
- Might need targets
- Fire events when activated
- Apply effects through subscriptions

## The Beautiful Simplicity

```go
// Player's turn
actions := character.GetAvailableActions()
// Show: [Rage, Attack, Fireball, Second Wind]

// Player clicks Fireball, selects target
character.ActivateAction("dnd5e:spell:fireball", "goblin-1")
// Consumes spell slot, fires event, damage happens

// Player clicks Rage
character.ActivateAction("dnd5e:feature:rage", "")  // No target
// Consumes rage use, fires event, resistance applied
```

**EVERYTHING IS AN ACTION!**

*flings spaghetti at ceiling - it sticks perfectly*