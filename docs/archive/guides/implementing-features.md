# Implementing Features - Step-by-Step Guide

## Quick Start: Your First Feature in 5 Minutes

```go
// 1. Define what your feature needs
type SecondWindInput struct{} // Nothing needed!

// 2. Create the feature as an Action
type SecondWind struct {
    id   string
    uses int
}

// 3. Implement Entity interface
func (s *SecondWind) GetID() string   { return s.id }
func (s *SecondWind) GetType() string { return "feature" }

// 4. Implement Action interface
func (s *SecondWind) CanActivate(ctx context.Context, owner Entity, input SecondWindInput) error {
    if s.uses <= 0 {
        return errors.New("no second wind uses remaining")
    }
    return nil
}

func (s *SecondWind) Activate(ctx context.Context, owner Entity, input SecondWindInput) error {
    s.uses--
    // Roll healing: 1d10 + fighter level
    healing := dice.New("1d10+4").Roll() // Assuming level 4
    
    // Publish healing event
    bus.Publish(events.New("healing.applied", map[string]any{
        "source": s.GetID(),
        "target": owner.GetID(),
        "amount": healing,
    }))
    return nil
}
```

That's it! You've created a working feature.

## Step-by-Step Feature Creation

### Step 1: Understand Your Feature

Ask yourself:
- **What triggers it?** (action, bonus action, reaction, passive)
- **What does it need?** (targets, positions, choices)
- **What resources does it use?** (uses per day, spell slots, etc.)
- **What effects does it create?** (damage, healing, conditions)
- **How long do effects last?** (instant, duration, concentration)

### Step 2: Define the Input Type

The input type captures what the feature needs to work:

```go
// No input needed (self-only abilities)
type RageInput struct{}

// Single target required
type StunningStrikeInput struct {
    Target core.Entity
}

// Multiple targets with position
type FireballInput struct {
    Center    spatial.Position
    Targets   []core.Entity  // Those in radius
    SlotLevel int            // Spell slot used
}

// Choice-based input
type FightingStyleInput struct {
    Choice string // "defense", "dueling", etc.
}
```

### Step 3: Create the Feature Structure

```go
type Feature struct {
    // Identity (required for Entity interface)
    id   string
    name string
    
    // Resources
    uses     int                    // Limited uses
    resource *resource.Resource     // Or use resource package
    
    // Configuration
    damage   string                  // Dice expression
    duration time.Duration           // Effect duration
    
    // State
    lastUsed time.Time              // For cooldowns
    active   bool                   // For toggleable features
}
```

### Step 4: Implement Validation (CanActivate)

This is where you check ALL prerequisites:

```go
func (f *Feature) CanActivate(ctx context.Context, owner Entity, input FeatureInput) error {
    // Check resources
    if f.uses <= 0 {
        return fmt.Errorf("%s has no uses remaining", f.name)
    }
    
    // Check cooldown
    if time.Since(f.lastUsed) < f.cooldown {
        remaining := f.cooldown - time.Since(f.lastUsed)
        return fmt.Errorf("%s on cooldown for %v", f.name, remaining)
    }
    
    // Check prerequisites
    if f.requiresConcentration && hasConcentration(owner) {
        return errors.New("already concentrating on another effect")
    }
    
    // Validate input
    if input.Target == nil {
        return errors.New("target required")
    }
    
    // Check range/reach
    if !inRange(owner, input.Target, f.range) {
        return fmt.Errorf("target out of range (%d ft)", f.range)
    }
    
    return nil
}
```

### Step 5: Implement Activation

This is where the feature does its work:

```go
func (f *Feature) Activate(ctx context.Context, owner Entity, input FeatureInput) error {
    // Consume resources
    f.uses--
    f.lastUsed = time.Now()
    
    // Roll any dice
    damage := dice.New(f.damage).Roll()
    
    // Publish events for effects
    bus.Publish(events.New("damage.dealt", map[string]any{
        "source": owner.GetID(),
        "target": input.Target.GetID(),
        "amount": damage,
        "type":   "fire",
    }))
    
    // Apply lasting effects
    if f.duration > 0 {
        effect := &FeatureEffect{
            id:       uuid.New(),
            source:   f.GetID(),
            duration: f.duration,
        }
        
        bus.Publish(events.New("effect.applied", map[string]any{
            "effect": effect,
            "target": input.Target.GetID(),
        }))
    }
    
    // Start concentration if needed
    if f.requiresConcentration {
        startConcentration(owner, f.GetID())
    }
    
    return nil
}
```

## Common Feature Patterns

### Pattern 1: Limited Use Feature

```go
type ActionSurge struct {
    id   string
    uses int
}

func (a *ActionSurge) CanActivate(ctx context.Context, owner Entity, input ActionSurgeInput) error {
    if a.uses <= 0 {
        return errors.New("no action surge uses remaining")
    }
    return nil
}

func (a *ActionSurge) Activate(ctx context.Context, owner Entity, input ActionSurgeInput) error {
    a.uses--
    
    // Grant extra action
    bus.Publish(events.New("action.granted", map[string]any{
        "recipient": owner.GetID(),
        "source":    a.GetID(),
        "type":      "action",
    }))
    
    return nil
}

// Reset on rest
func (a *ActionSurge) OnShortRest() {
    a.uses = 1
}
```

### Pattern 2: Resource-Based Feature

```go
type LayOnHands struct {
    id   string
    pool *resource.Resource
}

func NewLayOnHands(level int) *LayOnHands {
    return &LayOnHands{
        id: uuid.New(),
        pool: resource.New(resource.Config{
            ID:       "lay-on-hands-pool",
            MaxValue: level * 5,
            Current:  level * 5,
        }),
    }
}

func (l *LayOnHands) CanActivate(ctx context.Context, owner Entity, input LayOnHandsInput) error {
    if input.Amount <= 0 {
        return errors.New("must heal at least 1 point")
    }
    if !l.pool.CanConsume(input.Amount) {
        return fmt.Errorf("only %d points remaining", l.pool.Current())
    }
    return nil
}

func (l *LayOnHands) Activate(ctx context.Context, owner Entity, input LayOnHandsInput) error {
    l.pool.Consume(input.Amount)
    
    bus.Publish(events.New("healing.applied", map[string]any{
        "source": l.GetID(),
        "target": input.Target.GetID(),
        "amount": input.Amount,
    }))
    
    return nil
}
```

### Pattern 3: Toggleable Feature

```go
type DefensiveStance struct {
    id     string
    active bool
}

func (d *DefensiveStance) CanActivate(ctx context.Context, owner Entity, input DefensiveStanceInput) error {
    // Can always toggle
    return nil
}

func (d *DefensiveStance) Activate(ctx context.Context, owner Entity, input DefensiveStanceInput) error {
    d.active = !d.active
    
    if d.active {
        // Apply defensive bonus
        bus.Publish(events.New("stance.entered", map[string]any{
            "stance": "defensive",
            "owner":  owner.GetID(),
            "ac_bonus": 2,
        }))
    } else {
        // Remove defensive bonus
        bus.Publish(events.New("stance.exited", map[string]any{
            "stance": "defensive",
            "owner":  owner.GetID(),
        }))
    }
    
    return nil
}
```

### Pattern 4: Reaction Feature

```go
type Riposte struct {
    id   string
    uses int
}

// Subscribe to miss events
func (r *Riposte) OnAttackMissed(e events.Event) {
    attacker := e.Data["attacker"].(core.Entity)
    defender := e.Data["defender"].(core.Entity)
    
    // Check if we can riposte
    if err := r.CanActivate(ctx, defender, RiposteInput{Target: attacker}); err == nil {
        // Ask player if they want to riposte (game-specific)
        if promptReaction(defender, "Use Riposte?") {
            r.Activate(ctx, defender, RiposteInput{Target: attacker})
        }
    }
}

func (r *Riposte) Activate(ctx context.Context, owner Entity, input RiposteInput) error {
    r.uses--
    
    // Make counter attack
    bus.Publish(events.New("attack.triggered", map[string]any{
        "attacker": owner.GetID(),
        "target":   input.Target.GetID(),
        "type":     "riposte",
    }))
    
    return nil
}
```

## Testing Your Features

### Unit Test Template

```go
func TestFeature_CanActivate(t *testing.T) {
    tests := []struct {
        name    string
        setup   func(*Feature)
        input   FeatureInput
        wantErr string
    }{
        {
            name: "successful activation",
            setup: func(f *Feature) {
                f.uses = 1
            },
            input: FeatureInput{Target: mockTarget},
            wantErr: "",
        },
        {
            name: "no uses remaining",
            setup: func(f *Feature) {
                f.uses = 0
            },
            input: FeatureInput{Target: mockTarget},
            wantErr: "no uses remaining",
        },
        {
            name: "missing target",
            setup: func(f *Feature) {
                f.uses = 1
            },
            input: FeatureInput{},
            wantErr: "target required",
        },
    }
    
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            feature := &Feature{id: "test-feature", uses: 1}
            tt.setup(feature)
            
            err := feature.CanActivate(ctx, owner, tt.input)
            
            if tt.wantErr == "" {
                require.NoError(t, err)
            } else {
                require.Error(t, err)
                assert.Contains(t, err.Error(), tt.wantErr)
            }
        })
    }
}
```

### Integration Test Template

```go
func TestFeature_WithEventBus(t *testing.T) {
    // Setup
    bus := events.NewBus()
    feature := &Feature{id: "test-feature", uses: 1}
    owner := &MockEntity{id: "owner-1"}
    target := &MockEntity{id: "target-1"}
    
    // Track events
    var damageEvents []events.Event
    bus.Subscribe("damage.dealt", func(e events.Event) {
        damageEvents = append(damageEvents, e)
    })
    
    // Activate feature
    err := feature.Activate(ctx, owner, FeatureInput{Target: target})
    require.NoError(t, err)
    
    // Verify events
    require.Len(t, damageEvents, 1)
    assert.Equal(t, "owner-1", damageEvents[0].Data["source"])
    assert.Equal(t, "target-1", damageEvents[0].Data["target"])
}
```

## Feature Lifecycles

### Short Rest Features
```go
func (f *Feature) OnShortRest() {
    f.uses = f.maxUses
}
```

### Long Rest Features
```go
func (f *Feature) OnLongRest() {
    f.uses = f.maxUses
    f.pool.SetCurrent(f.pool.Max())
}
```

### Level-Based Features
```go
func (f *Feature) OnLevelUp(newLevel int) {
    if newLevel >= 5 {
        f.maxUses = 2
    }
    if newLevel >= 11 {
        f.damage = "2d8" // Upgrade damage
    }
}
```

## Debugging Features

### Event Logging
```go
func (f *Feature) Activate(ctx context.Context, owner Entity, input FeatureInput) error {
    // Log activation
    log.Printf("[%s] Activating %s: owner=%s, target=%s",
        time.Now().Format(time.RFC3339),
        f.name,
        owner.GetID(),
        input.Target.GetID(),
    )
    
    // ... rest of activation
}
```

### State Inspection
```go
func (f *Feature) String() string {
    return fmt.Sprintf("Feature{id=%s, uses=%d/%d, active=%v}",
        f.id, f.uses, f.maxUses, f.active)
}
```

### Event Tracing
```go
// Subscribe to all feature events for debugging
bus.Subscribe("feature.*", func(e events.Event) {
    log.Printf("Feature Event: %s - %+v", e.Type, e.Data)
})
```

## Checklist for New Features

- [ ] Input type defined with required fields
- [ ] Feature struct with necessary state
- [ ] Entity interface implemented (GetID, GetType)
- [ ] CanActivate validates all prerequisites
- [ ] Activate consumes resources before effects
- [ ] Events published for all effects
- [ ] Duration effects have cleanup
- [ ] Concentration handled if needed
- [ ] Unit tests for validation logic
- [ ] Integration tests with event bus
- [ ] Rest/reset logic implemented
- [ ] Documentation comments added

## Common Mistakes to Avoid

### 1. Resource Consumption Before Validation
```go
// WRONG
func (f *Feature) Activate(...) error {
    f.uses-- // Don't do this first!
    if input.Target == nil {
        return errors.New("no target") // Uses already consumed!
    }
}

// RIGHT
func (f *Feature) CanActivate(...) error {
    if input.Target == nil {
        return errors.New("no target")
    }
    if f.uses <= 0 {
        return errors.New("no uses")
    }
    return nil
}
```

### 2. Forgetting Effect Cleanup
```go
// WRONG
func (f *Feature) Activate(...) error {
    applyEffect(target, "stunned")
    // Effect lasts forever!
}

// RIGHT
func (f *Feature) Activate(...) error {
    applyEffect(target, "stunned")
    scheduleRemoval(target, "stunned", 1*time.Minute)
}
```

### 3. Direct State Modification
```go
// WRONG
func (f *Feature) Activate(...) error {
    target.(*Character).HP -= damage // Type assertion and direct mutation!
}

// RIGHT
func (f *Feature) Activate(...) error {
    bus.Publish(events.New("damage.dealt", map[string]any{
        "target": target.GetID(),
        "amount": damage,
    }))
}
```

## Next Steps

1. Start with simple features (no input, limited uses)
2. Add resource management (pools, slots)
3. Implement duration effects
4. Add reaction features
5. Create composite features

Remember: Features are just Actions with specific patterns. Master the pattern, and you can implement any RPG feature!