# Journey 014: Event Bus Modifier System - Making Events Actually Modify Actions

**Date**: 2025-08-13  
**Context**: After successfully implementing type-safe event routing with `core.Ref`, we realized we'd lost sight of the core purpose - events need to modify game actions like damage and attack rolls.

## The Problem

We built a beautiful type-safe event bus that could route events to handlers based on ref pointers. But it was read-only - handlers could receive events but couldn't modify outcomes. 

For example, Rage should:
- Add +2 damage to melee attacks (damage dealt)
- Give resistance to physical damage (damage taken)
- Give advantage on Strength checks

But our event bus just passed events through. There was no mechanism for handlers to say "add 2 to that damage" or "this damage should be halved."

## The Journey

### First Realization: We Need Mutable Context

The event bus was designed for notification, not modification. We needed events to carry a mutable context that handlers could modify:

```go
// Original - just notification
type Event interface {
    EventRef() *core.Ref
}

// New - carries mutable context
type Event interface {
    EventRef() *core.Ref
    Context() *EventContext  // Handlers can add modifiers here
}
```

### Second Realization: Modifiers Are Just Data

We started listing modifier types (resistance, vulnerability, advantage, reroll, etc.) and realized they're all just different ways to modify a value:

- **Additive**: +2 damage, -1 AC
- **Multiplicative**: x0.5 (resistance), x2 (vulnerability)  
- **Dice**: +1d4 (Bless)
- **Flags**: advantage, disadvantage, immune
- **Override**: replace the value entirely

The modifier doesn't need to know HOW it's applied - that's the event's job.

### Third Realization: String Soup is a Code Smell

Our first design had strings everywhere:
- Modifier types as strings
- Damage types as strings  
- Targets as strings
- Sources as strings

This felt wrong. Every string is a potential typo, a runtime error waiting to happen.

### Fourth Realization: Base vs Rulebook Responsibilities

The key insight: the base (rpg-toolkit) should define HOW to modify values, while rulebooks define WHAT those modifications mean.

For example:
- Base says: "here's a multiplicative modifier of 0.5"
- D&D 5e rulebook says: "that's resistance, apply it when damage type matches"
- Pathfinder rulebook might interpret it differently

## The Solution

### Base Layer (rpg-toolkit/events)

```go
// Event interface - simple and universal
type Event interface {
    EventRef() *core.Ref
    Context() *EventContext
}

// EventContext - mutable container for modifiers
type EventContext struct {
    modifiers []Modifier
    data      map[string]interface{}
    mu        sync.RWMutex  // Thread-safe
}

// Modifier interface - just data
type Modifier interface {
    Source() string      // Who added this
    Type() string        // How to modify
    Priority() int       // When to apply
    Value() interface{}  // The modification data
}

// Suggested modifier types (not enforced)
const (
    ModifierTypeAdditive       = "additive"
    ModifierTypeMultiplicative = "multiplicative"
    ModifierTypeDice          = "dice"
    ModifierTypeFlag          = "flag"
    ModifierTypeOverride      = "override"
)
```

### Rulebook Layer (rulebooks/dnd5e)

```go
// Concrete event with D&D specific knowledge
type DamageEvent struct {
    ref        *core.Ref
    context    *events.EventContext
    BaseDamage int
    DamageType string  // "slashing", "fire", etc.
    Attacker   core.Entity
    Defender   core.Entity
}

// Implements Event interface
func (e *DamageEvent) EventRef() *core.Ref           { return e.ref }
func (e *DamageEvent) Context() *events.EventContext { return e.context }

// Knows how to apply modifiers the D&D way
func (e *DamageEvent) CalculateFinalDamage() int {
    damage := e.BaseDamage
    
    for _, mod := range e.Context().GetModifiers() {
        switch mod.Type() {
        case events.ModifierTypeAdditive:
            damage += mod.Value().(int)
        case events.ModifierTypeMultiplicative:
            // D&D interprets 0.5 as resistance, 2.0 as vulnerability
            damage = int(float64(damage) * mod.Value().(float64))
        case events.ModifierTypeFlag:
            if mod.Value() == "immune" {
                return 0  // D&D rule: immunity negates all damage
            }
        }
    }
    
    return damage
}
```

### Feature Implementation (Rage)

```go
func (rage *RageFeature) OnDamageDealt(event events.Event) error {
    // Add flat damage bonus
    event.Context().AddModifier(&SimpleModifier{
        source:   "Rage",
        modType:  events.ModifierTypeAdditive,
        priority: 20,
        value:    2,  // +2 damage
    })
    return nil
}

func (rage *RageFeature) OnDamageTaken(event events.Event) error {
    // Check if it's physical damage (rulebook-specific check)
    if dmgEvent, ok := event.(*DamageEvent); ok {
        if isPhysical(dmgEvent.DamageType) {
            // Add resistance (half damage)
            event.Context().AddModifier(&SimpleModifier{
                source:   "Rage",
                modType:  events.ModifierTypeMultiplicative,
                priority: 100,  // Resistance applies late
                value:    0.5,
            })
        }
    }
    return nil
}
```

## Key Decisions

1. **Mutable Context Pattern**: Events carry a mutable context that accumulates modifiers as they pass through handlers.

2. **Modifiers as Data**: Modifiers don't contain logic - they're just data. The event decides how to apply them.

3. **String Types with Constants**: We use strings for flexibility but provide constants to prevent typos.

4. **Clear Separation**: 
   - rpg-toolkit: Provides infrastructure (Event, Context, Modifier interfaces)
   - Rulebooks: Define concrete events and interpretation rules
   - Features: Just add modifiers without knowing implementation

5. **Priority System**: Modifiers have priorities to control application order (e.g., resistance applies after flat bonuses).

## What This Enables

- **Rage** can add damage bonuses and resistance
- **Bless** can add 1d4 to attack rolls  
- **Shield** spell can add +5 AC reactively
- **Critical Hits** can double damage
- **Magic Weapons** can add flat bonuses
- **Vulnerabilities** can double damage from specific types

All without the features knowing HOW these modifications are applied - they just declare their intent through modifiers.

## Lessons Learned

1. **Start with the goal**: We got so focused on type safety that we forgot events need to modify outcomes.

2. **Strings aren't always bad**: With constants and clear documentation, strings provide flexibility without significant downsides.

3. **Separation of concerns is key**: The base shouldn't know about D&D concepts like "resistance" - that's rulebook knowledge.

4. **Mutable shared state is OK**: When properly synchronized and scoped to event handling, mutable context is the simplest solution.

## Next Steps

1. Implement the base Event/Context/Modifier system in rpg-toolkit
2. Create D&D 5e specific events in the rulebook
3. Implement a few features (Rage, Bless) to validate the design
4. Add comprehensive tests showing modifier interaction
5. Document the contract between publishers and subscribers

## Open Questions

1. Should modifiers be able to cancel each other? (advantage + disadvantage = normal)
2. How do we handle stacking rules? (multiple rage bonuses)
3. Should events validate modifier types or just ignore unknown ones?
4. Do we need modifier conditions? (only applies if X)

These can be answered as we implement and see real usage patterns.