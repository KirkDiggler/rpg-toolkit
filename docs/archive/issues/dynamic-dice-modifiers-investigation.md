# Investigation: Dynamic Dice Modifiers - Fresh Rolls for Persistent Effects

## Problem Statement

The current dice modifier system caches rolls after first use. This works for single-use modifiers but breaks for persistent effects that apply to multiple rolls. For example:

- **Bless**: Adds 1d4 to attack rolls and saving throws
- **Bardic Inspiration**: Adds 1d6/1d8/1d10/1d12 to various rolls
- **Guidance**: Adds 1d4 to ability checks
- **Sneak Attack**: Adds multiple d6 to damage

Currently, our modifier system assumes numeric modifiers. We need a way to express and handle dice expressions as modifiers.

## Current Behavior

```go
// Current dice.Roll implementation caches the result
type Roll struct {
    rolled bool    // Has this been rolled?
    result int     // Cached result
}

func (r *Roll) GetValue() int {
    if !r.rolled {
        r.roll()  // Roll happens ONCE
    }
    return r.result  // Same value returned every time
}

// Problem: Bless creates one d4 modifier that's reused
blessModifier := dice.D4(1)
// Attack 1: rolls 3 (cached)
// Attack 2: returns 3 (cached) - WRONG! Should roll fresh
// Attack 3: returns 3 (cached) - WRONG! Should roll fresh
```

## Desired Behavior

```go
// Need: Fresh rolls for persistent effects
type DiceModifier interface {
    ModifierValue
    Fresh() bool  // Indicates if this should roll fresh each time
}

// Or: Factory pattern for dice modifiers
type DiceModifierFactory func() ModifierValue

// Bless effect provides a factory, not a pre-rolled value
blessEffect := func() ModifierValue {
    return dice.D4(1)  // Fresh d4 each time
}

// Attack 1: rolls 3 
// Attack 2: rolls 1 (fresh roll!)
// Attack 3: rolls 4 (fresh roll!)
```

## Investigation Areas

### 1. Fresh Roll Mechanism
- How do we distinguish between one-time and persistent modifiers?
- Should effects provide modifier factories instead of modifier values?
- Can we maintain backward compatibility with cached rolls?

### 2. Event Integration
- How do dice modifiers flow through events?
- When exactly are the dice rolled?
- How do we ensure fresh rolls each time?

### 3. Modifier Stacking
- How do multiple dice modifiers combine?
- Do we need rules for dice stacking (like advantage/disadvantage)?
- How do dice and numeric modifiers mix?

### 4. Implementation Patterns

Consider these approaches:

**Option A: Modifier Factory Pattern**
```go
// Effects provide factories, not values
type ModifierFactory func() *Modifier

func (bless *BlessEffect) Apply(bus EventBus) error {
    factory := func() *Modifier {
        return events.NewModifier(
            "blessed",
            events.ModifierAttackBonus,
            dice.D4(1),  // Fresh roll each time
            100,
        )
    }
    // Register factory instead of modifier
}
```

**Option B: Non-Caching Dice Roll**
```go
// Add a Fresh() or NoCache() option to dice.Roll
type Roll struct {
    cacheable bool  // Whether to cache the result
}

blessModifier := dice.D4(1).NoCache()
```

**Option C: Lazy Modifier Pattern**
```go
// Modifier that creates its value on demand
type LazyModifier struct {
    source string
    typ    ModifierType
    factory func() ModifierValue
}

func (l *LazyModifier) ModifierValue() ModifierValue {
    return l.factory()  // Fresh each time
}
```

### 5. Integration with Effect System

How do effects like Bless specify dice modifiers?

```go
type BlessEffect struct {
    EffectCore
    
    // Implements DiceModifier behavior?
    diceBonus string // "1d4"
}

func (b *BlessEffect) Apply(bus EventBus) error {
    b.Subscribe(bus, EventBeforeAttackRoll, func(e Event) error {
        // Add dice modifier to event
        e.AddDiceModifier("1d4", "bless")
        return nil
    })
}
```

## Success Criteria

1. Effects can add dice expressions as modifiers
2. Dice are rolled fresh for each application
3. Clear API for effects to specify dice modifiers
4. Integration with existing modifier system
5. Efficient and performant implementation

## Next Steps

1. Research how other systems handle this (Roll20 API, Foundry VTT, etc.)
2. Prototype different approaches
3. Test with common use cases (Bless, Sneak Attack, etc.)
4. Document decision and implementation
5. Update effect composition pattern if needed

## Related Issues

- #29: Proficiency system (uses modifiers)
- #30: Resource management (some resources add dice)
- Effect Composition ADR (needs DiceModifier behavior)