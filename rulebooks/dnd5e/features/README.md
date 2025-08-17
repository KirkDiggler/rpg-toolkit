# D&D 5e Features Package

This package implements D&D 5e character features (rage, second wind, action surge, etc.) as Actions that apply self-contained Conditions through events.

## Core Architecture

### The Event-Driven Condition Pattern

Features don't directly modify characters or track active state. Instead:

1. **Features publish condition events** when activated
2. **Characters subscribe** to condition events targeting them  
3. **Conditions are self-contained** - they manage their own lifecycle
4. **Conditions handle all mechanics** - duration, modifiers, removal

This creates beautiful separation:
- Features only know about activation rules and resource consumption
- Conditions encapsulate all the ongoing effects and rules
- Characters just host conditions without knowing their mechanics

## The Flow

```
Player: "I rage!"
  ↓
Feature.Activate()
  → Consumes rage use
  → Publishes ConditionAppliedEvent{Target: barbarian, Type: "raging", Data: {level: 5}}
  → Done! Feature doesn't track anything else
  ↓
Character.OnConditionApplied()
  → Loads condition: LoadConditionByRef("dnd5e:conditions:raging")
  → Calls: condition.Apply(eventBus, character)
  → Adds to condition list
  ↓
RagingCondition.Apply()
  → Subscribes to AttackEvent (track if I attacked)
  → Subscribes to DamageReceivedEvent (track if I was hit)
  → Subscribes to RoundEndEvent (check if rage continues)
  → Subscribes to combat events (apply modifiers)
  ↓
[During combat...]
  → Attack happens: RagingCondition adds damage bonus
  → Damage received: RagingCondition applies resistance
  → Round ends: RagingCondition checks if it should continue
  ↓
[When rage ends...]
RagingCondition.OnRoundEnd()
  → Didn't attack or get hit? Rage ends
  → 10 rounds passed? Rage ends
  → Publishes: ConditionRemovedEvent{Target: barbarian, Type: "raging"}
  ↓
Character.OnConditionRemoved()
  → Removes from condition list
  → Condition cleans up its subscriptions
```

## Implementation Structure

### Constants and Types

```go
// features/constants.go

// Event refs - typed constants for all events
var (
    EventRefConditionApplied  = core.MustParseRef("dnd5e:events:condition_applied")
    EventRefConditionRemoved  = core.MustParseRef("dnd5e:events:condition_removed")
    EventRefRoundEnd         = core.MustParseRef("dnd5e:events:round_end")
)

// Condition refs - typed constants for all conditions
var (
    ConditionRefRaging       = core.MustParseRef("dnd5e:conditions:raging")
    ConditionRefSecondWind   = core.MustParseRef("dnd5e:conditions:second_wind")
    ConditionRefActionSurge  = core.MustParseRef("dnd5e:conditions:action_surge")
)

// Typed errors for better error handling
var (
    ErrNoUsesRemaining = errors.New("no rage uses remaining")
    ErrAlreadyRaging   = errors.New("already raging")
)
```

### Feature Definition

```go
// features/rage.go
type Rage struct {
    id    string
    uses  int
    level int
    bus   *events.Bus
    
    // State management
    mu          sync.RWMutex
    currentUses int
}

// CanActivate just returns nil - all validation happens in Activate
// This satisfies the core.Action interface but we don't use it
func (r *Rage) CanActivate(_ context.Context, _ core.Entity, _ FeatureInput) error {
    return nil  // All validation in Activate for simpler flow
}

func (r *Rage) Activate(ctx context.Context, owner core.Entity, input FeatureInput) error {
    r.mu.Lock()
    defer r.mu.Unlock()
    
    // All validation here - single place for all checks
    if r.currentUses <= 0 {
        return ErrNoUsesRemaining
    }
    
    // Check if already has raging condition
    if owner.HasCondition(ConditionRefRaging.String()) {
        return ErrAlreadyRaging
    }
    
    // Consume use
    r.currentUses--
    
    // Publish typed condition event
    return r.bus.Publish(&ConditionAppliedEvent{
        Target:    owner.GetID(),
        Condition: ConditionRefRaging.String(),
        Source:    r.GetID(),
        Data: map[string]any{
            "level": r.level,  // For damage bonus calculation
        },
    })
}
```

### Condition Definition

```go
// features/rage_condition.go (same package as feature)
type RagingCondition struct {
    owner        string
    level        int
    ticksRemaining int
    bus          *events.Bus
    
    // Track state for rage rules
    attackedThisRound bool
    wasHitThisRound   bool
    
    // Subscriptions for cleanup
    subscriptions []string
}

func (rc *RagingCondition) Apply(bus *events.Bus, owner core.Entity) error {
    rc.bus = bus
    rc.owner = owner.GetID()
    rc.ticksRemaining = 10
    
    // Subscribe to combat events using typed refs
    rc.subscriptions = []string{
        bus.SubscribeWithFilter(dnd5e.EventRefAttack, rc.onAttack, rc.filterMyAttacks),
        bus.SubscribeWithFilter(dnd5e.EventRefDamageReceived, rc.onDamageReceived, rc.filterMyDamage),
        bus.Subscribe(EventRefRoundEnd, rc.onRoundEnd),
    }
    
    return nil
}

func (rc *RagingCondition) onAttack(e interface{}) error {
    attack := e.(*AttackEvent)
    
    // Track that we attacked
    rc.attackedThisRound = true
    
    // Add damage bonus to STR melee attacks
    if attack.IsMelee && attack.Ability == AbilityStrength {
        ctx := attack.Context()
        damageBonus := rc.calculateDamageBonus()
        ctx.AddModifier(events.NewSimpleModifier(
            ModifierSourceRage,
            ModifierTypeAdditive,
            ModifierTargetDamage,
            200,
            damageBonus,
        ))
    }
    
    return nil
}

func (rc *RagingCondition) onDamageReceived(e interface{}) error {
    damage := e.(*DamageReceivedEvent)
    
    // Track that we were hit
    rc.wasHitThisRound = true
    
    // Apply resistance to physical damage
    if isPhysicalDamage(damage.DamageType) {
        ctx := damage.Context()
        ctx.AddModifier(events.NewSimpleModifier(
            ModifierSourceRage,
            ModifierTypeResistance,
            ModifierTargetDamage,
            100,
            0.5,
        ))
    }
    
    return nil
}

func (rc *RagingCondition) onRoundEnd(e interface{}) error {
    // Check if rage continues
    if !rc.attackedThisRound && !rc.wasHitThisRound {
        return rc.remove("You didn't attack or take damage")
    }
    
    rc.ticksRemaining--
    if rc.ticksRemaining <= 0 {
        return rc.remove("Rage duration expired")
    }
    
    // Reset for next round
    rc.attackedThisRound = false
    rc.wasHitThisRound = false
    
    return nil
}

func (rc *RagingCondition) remove(reason string) error {
    // Unsubscribe from all events
    for _, sub := range rc.subscriptions {
        rc.bus.Unsubscribe(sub)
    }
    
    // Publish typed removal event
    return rc.bus.Publish(&ConditionRemovedEvent{
        Target:    rc.owner,
        Condition: ConditionRefRaging.String(),
        Reason:    reason,
    })
}
```

## Character Integration

The character package provides the routing to load conditions from various packages:

```go
// character/conditions.go
func LoadConditionByRef(ref core.Ref, data map[string]any, bus *events.Bus) (Condition, error) {
    switch ref {
    // Feature-specific conditions (from features package)
    case features.ConditionRefRaging:
        return features.NewRagingCondition(data, bus)
    case features.ConditionRefSecondWind:
        return features.NewSecondWindCondition(data, bus)
    case features.ConditionRefActionSurge:
        return features.NewActionSurgeCondition(data, bus)
    
    // Standard D&D conditions (from conditions package)
    case conditions.ConditionRefPoisoned:
        return conditions.NewPoisonedCondition(data, bus)
    case conditions.ConditionRefGrappled:
        return conditions.NewGrappledCondition(data, bus)
    case conditions.ConditionRefStunned:
        return conditions.NewStunnedCondition(data, bus)
    
    // Spell conditions (from spells package)
    case spells.ConditionRefBlessed:
        return spells.NewBlessedCondition(data, bus)
    case spells.ConditionRefHexed:
        return spells.NewHexedCondition(data, bus)
    
    default:
        return nil, fmt.Errorf("unknown condition: %s", ref)
    }
}

// Character subscribes to condition events
func (c *Character) OnConditionApplied(e interface{}) error {
    event := e.(*ConditionAppliedEvent)
    
    // Only handle conditions targeting this character
    if event.Target != c.GetID() {
        return nil
    }
    
    // Load and apply the condition
    condition, err := LoadConditionByRef(event.Condition, event.Data, c.bus)
    if err != nil {
        return err
    }
    
    if err := condition.Apply(c.bus, c); err != nil {
        return err
    }
    
    // Add to our condition list
    c.conditions[event.Condition] = condition
    
    return nil
}

func (c *Character) OnConditionRemoved(e interface{}) error {
    event := e.(*ConditionRemovedEvent)
    
    if event.Target != c.GetID() {
        return nil
    }
    
    // Remove from our list
    delete(c.conditions, event.Condition)
    
    return nil
}
```

## Key Benefits

### 1. Clean Separation of Concerns
- **Features**: Handle activation, resource consumption
- **Conditions**: Handle ongoing effects, duration, modifiers
- **Characters**: Just host conditions, don't know mechanics

### 2. Self-Contained Logic
Each condition knows its complete ruleset:
- When it ends (duration, triggers)
- What modifiers it provides
- What events it cares about
- How to clean itself up

### 3. Persistence-Friendly
```go
// Save: Character just saves its condition list
characterData.Conditions = []ConditionData{
    {Ref: "dnd5e:conditions:raging", TicksRemaining: 7, Data: {level: 5}},
    {Ref: "dnd5e:conditions:blessed", TicksRemaining: 3},
}

// Load: Character reapplies conditions
for _, cond := range characterData.Conditions {
    condition := LoadConditionByRef(cond.Ref, cond.Data, bus)
    condition.Apply(bus, character)
    character.conditions[cond.Ref] = condition
}
```

### 4. Extensible
Adding new features/conditions is straightforward:
1. Create the feature (handles activation)
2. Create its condition (handles effects)
3. Register in LoadConditionByRef
4. Done!

## Design Principles

1. **Features don't track active state** - they publish conditions
2. **Conditions are self-contained** - complete lifecycle management
3. **Event-driven application** - no direct manipulation
4. **Single responsibility** - each part does one thing well
5. **Package cohesion** - related conditions live with their features
6. **Constants everywhere** - typed refs for events and conditions, no strings
7. **Single validation point** - CanActivate returns nil, all checks in Activate

## Testing Patterns

### Testing Features
```go
func TestRage_PublishesCondition(t *testing.T) {
    bus := events.NewBus()
    rage := NewRage(3, 5, bus)
    barbarian := &mockCharacter{id: "conan"}
    
    // Subscribe to condition events
    var appliedEvent *ConditionAppliedEvent
    bus.Subscribe("dnd5e:events:condition_applied", func(e interface{}) error {
        appliedEvent = e.(*ConditionAppliedEvent)
        return nil
    })
    
    // Activate rage
    err := rage.Activate(ctx, barbarian, FeatureInput{})
    require.NoError(t, err)
    
    // Verify condition event was published
    assert.NotNil(t, appliedEvent)
    assert.Equal(t, "dnd5e:conditions:raging", appliedEvent.Condition)
    assert.Equal(t, "conan", appliedEvent.Target)
}
```

### Testing Conditions
```go
func TestRagingCondition_EndsWithoutCombat(t *testing.T) {
    bus := events.NewBus()
    condition := NewRagingCondition(map[string]any{"level": 5}, bus)
    barbarian := &mockCharacter{id: "conan"}
    
    // Apply condition
    condition.Apply(bus, barbarian)
    
    // Subscribe to removal events
    var removedEvent *ConditionRemovedEvent
    bus.Subscribe("dnd5e:events:condition_removed", func(e interface{}) error {
        removedEvent = e.(*ConditionRemovedEvent)
        return nil
    })
    
    // Publish round end without any attacks
    bus.Publish(&RoundEndEvent{})
    
    // Verify rage ended
    assert.NotNil(t, removedEvent)
    assert.Equal(t, "You didn't attack or take damage", removedEvent.Reason)
}
```

## Current Implementation Status

✅ **Architecture Designed**:
- Event-driven condition application
- Self-contained condition lifecycle
- Character routing pattern

🚧 **Next Steps**:
- Implement ConditionAppliedEvent and ConditionRemovedEvent
- Refactor Rage to publish conditions instead of tracking state
- Create RagingCondition with full rage mechanics
- Update Character to subscribe to condition events
- Add LoadConditionByRef routing

## FAQ

**Q: Why separate features and conditions?**
A: Features handle the "can I do this?" question. Conditions handle "what happens while it's active?"

**Q: Where do spell conditions go?**
A: With their spells! A Bless spell would have a BlessedCondition in the spells package.

**Q: How do conditions from items work?**
A: Same pattern - items publish condition events, characters apply them.

**Q: Can conditions interact with each other?**
A: Yes, through events. A condition can subscribe to other condition events if needed.

**Q: What about conditions that don't come from features?**
A: Standard conditions (poisoned, grappled) live in the conditions package and work the same way.