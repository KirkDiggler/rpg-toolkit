# Targeting and Event Filtering

## The Problem

Features are hearing ALL events in the room/game! We need to filter for relevance.

## Features Need Target Information

```go
type Feature interface {
    // ... existing methods ...
    
    // Some features need targets
    NeedsTarget() bool
    ValidateTarget(target core.Entity) error
    ActivateWithTarget(target core.Entity) error
}
```

## Different Activation Patterns

```go
// Rage - no target needed
func (r *RageFeature) NeedsTarget() bool {
    return false
}

func (r *RageFeature) Activate() error {
    // Affects self only
    return r.activateSelf()
}

// Sneak Attack - needs a target
func (s *SneakAttackFeature) NeedsTarget() bool {
    return true
}

func (s *SneakAttackFeature) ValidateTarget(target core.Entity) error {
    if target == nil {
        return fmt.Errorf("sneak attack requires a target")
    }
    
    // Check if target is valid enemy
    if target.GetType() != "creature" {
        return fmt.Errorf("can only sneak attack creatures")
    }
    
    // Check range
    if distance(s.owner, target) > 5 {
        return fmt.Errorf("target is too far away")
    }
    
    return nil
}

func (s *SneakAttackFeature) ActivateWithTarget(target core.Entity) error {
    if err := s.ValidateTarget(target); err != nil {
        return err
    }
    
    s.currentTarget = target
    s.isActive = true
    
    // Fire targeted event
    event := events.NewGameEvent("feature.activate", s.owner, target)
    event.Context().Set("feature_ref", SneakAttackRef)
    
    return s.eventBus.Publish(context.Background(), event)
}
```

## Event Filtering - Only Listen to Relevant Events

```go
// Rage only cares about damage to SELF
func (r *RageFeature) apply(f *features.SimpleFeature, bus events.EventBus) error {
    // Subscribe to damage events
    f.Subscribe(bus, events.EventBeforeTakeDamage, 50, 
        func(ctx context.Context, e events.Event) error {
            // FILTER: Only care if WE are taking damage
            if e.Target().GetID() != f.Owner().GetID() {
                return nil  // Not our damage, ignore
            }
            
            // FILTER: Only apply if rage is active
            if !r.isActive {
                return nil
            }
            
            // Now we know it's our damage while raging
            damageType, _ := e.Context().Get("damage_type")
            if dt, ok := damageType.(string); ok {
                if dt == "slashing" || dt == "piercing" || dt == "bludgeoning" {
                    // Apply resistance
                    damage, _ := e.Context().Get("damage")
                    if dmg, ok := damage.(int); ok {
                        e.Context().Set("damage", dmg/2)
                    }
                }
            }
            
            return nil
        })
    
    // Subscribe to OUR attack events for damage bonus
    f.Subscribe(bus, events.EventOnDamageRoll, 50,
        func(ctx context.Context, e events.Event) error {
            // FILTER: Only care if WE are attacking
            if e.Source().GetID() != f.Owner().GetID() {
                return nil
            }
            
            // FILTER: Only if raging
            if !r.isActive {
                return nil
            }
            
            // Add rage damage bonus
            e.Context().AddModifier(events.NewModifier(
                events.ModifierDamageBonus,
                "Rage",
                dice.NewFlat(r.getRageDamageBonus()),
            ))
            
            return nil
        })
    
    return nil
}

// Sneak Attack only cares about attacks on current target
func (s *SneakAttackFeature) apply(f *features.SimpleFeature, bus events.EventBus) error {
    f.Subscribe(bus, events.EventOnDamageRoll, 50,
        func(ctx context.Context, e events.Event) error {
            // FILTER: Only our attacks
            if e.Source().GetID() != f.Owner().GetID() {
                return nil
            }
            
            // FILTER: Only if we have a target
            if s.currentTarget == nil {
                return nil
            }
            
            // FILTER: Only attacks on our chosen target
            if e.Target().GetID() != s.currentTarget.GetID() {
                return nil
            }
            
            // FILTER: Only once per turn
            if s.usedThisTurn {
                return nil
            }
            
            // Add sneak attack damage
            e.Context().AddModifier(events.NewModifier(
                events.ModifierDamageBonus,
                "Sneak Attack",
                dice.NewDice(s.getSneakAttackDice(), 6),
            ))
            
            s.usedThisTurn = true
            s.dirty = true
            
            return nil
        })
    
    return nil
}
```

## Room-Level Event Filtering

```go
// Events could be scoped to rooms
type Room struct {
    id       string
    entities map[string]core.Entity
    eventBus events.EventBus  // Room-specific bus
}

// Only entities in the room hear events
func (r *Room) PublishEvent(event events.Event) {
    // Only goes to this room's bus
    r.eventBus.Publish(context.Background(), event)
}

// Or events could include room info
type CombatEvent struct {
    events.GameEvent
    RoomID string  // Which room this happened in
}

// Features check room
func (f *SomeFeature) handleEvent(ctx context.Context, e events.Event) error {
    if combatEvent, ok := e.(*CombatEvent); ok {
        // Only care about events in our room
        if combatEvent.RoomID != f.owner.GetRoomID() {
            return nil
        }
    }
    // ...
}
```

## API for Targeted Activation

```go
// Client sends target with activation
POST /api/activate
{
    "ref": "dnd5e:feature:sneak_attack",
    "target_id": "goblin-1"
}

// Server handles targeting
func HandleActivate(req ActivateRequest) error {
    feature, err := character.GetFeature(req.Ref)
    if err != nil {
        return err
    }
    
    // Check if feature needs target
    if feature.NeedsTarget() {
        if req.TargetID == "" {
            return fmt.Errorf("%s requires a target", feature.Name())
        }
        
        target := room.GetEntity(req.TargetID)
        if target == nil {
            return fmt.Errorf("target not found")
        }
        
        return feature.ActivateWithTarget(target)
    }
    
    // No target needed
    return feature.Activate()
}
```

## The Key Pattern

**Features filter events to only what they care about:**

1. **Identity Filter** - Is this about me? My target?
2. **State Filter** - Am I active? Is condition met?
3. **Scope Filter** - Is this in my room/range?
4. **Frequency Filter** - Have I already triggered this turn?

This way features don't react to EVERYTHING, just what's relevant to them!