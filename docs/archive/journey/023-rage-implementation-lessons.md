# Journey 023: The Rage Implementation That Taught Us Architecture

## The Starting Point

We set out to implement D&D 5e's rage feature. Should be simple, right? Barbarian gets angry, gets some bonuses, hits things harder. About 100 lines of code max.

## The Discovery Phase

### Problem 1: The Deadlock
Our first attempt had rage trying to unsubscribe itself when it ended:
```go
func (r *RagingCondition) handleTurnEnd(e Event) error {
    if !r.attackedThisRound {
        // Rage ends - unsubscribe ourself
        r.bus.Unsubscribe(r.subscriptionID) // DEADLOCK!
    }
}
```

The event bus held a read lock while calling handlers. Handlers couldn't get a write lock to unsubscribe. Classic deadlock.

### Solution: Deferred Operations (PR #220)
We added deferred operations to the event bus:
```go
func (r *RagingCondition) handleTurnEnd(e Event) *DeferredAction {
    if !r.attackedThisRound {
        return events.NewDeferredAction().Unsubscribe(r.subscriptionID)
    }
    return nil
}
```

This was genuinely good architecture. Events complete, then deferred actions execute. Clean, safe, no deadlocks.

## The Unraveling

### Problem 2: String Explosion
```go
events.NewSimpleModifier("advantage", "strength_check", nil)
events.NewSimpleModifier("resistance", "damage.bludgeoning", nil)
core.ParseString("feature:barbarian:rage:ended")
```

Magic strings everywhere. No type safety. No IDE help. Easy to typo.

### Problem 3: Not Using What We Have
We have `mechanics/effects` with `effects.Core` that handles:
- Modifier management
- Event subscriptions
- Lifecycle (apply/tick/remove)

But we ignored it and tried to build everything from scratch in the rage feature.

### Problem 4: Identity Crisis
Is rage a feature or a condition?
- `rage.go` - The feature that tracks uses
- `rage_condition.go` - The condition that modifies things

We were mixing activation (feature responsibility) with modification (effect responsibility).

## The Realizations

### 1. Core Should Define Types, Not Implementation
```go
// Good - core defines the type
type ModifierType string
const ModifierAdvantage ModifierType = "advantage"

// Bad - core implements the logic
func ApplyAdvantage(roll *Roll) { ... }
```

### 2. Conditions Are Just Effects With Lifecycle
We don't need a separate "condition" concept. An effect that:
- Has duration
- Can be removed
- Modifies stats
- Responds to events

That's a condition!

### 3. Features Activate, Effects Modify
Clean separation:
```go
// Feature manages resources and activation
type RageFeature struct {
    uses int
}

func (f *RageFeature) Activate() error {
    effect := NewRageEffect()
    return effectManager.Apply(effect)
}

// Effect provides the modifications
type RageEffect struct {
    *effects.Core
}

func (e *RageEffect) Modifiers() []Modifier {
    // Return the actual rage bonuses
}
```

## The Lessons

### What Went Right
1. **Deferred operations** - Solved a real architectural problem
2. **Event-driven design** - The pattern is sound
3. **Test-first** - Our tests revealed the design issues

### What Went Wrong
1. **Ignored existing infrastructure** - effects.Core was there all along
2. **String magic** - No type safety made everything fragile
3. **Mixed responsibilities** - Features trying to be effects
4. **Rushed implementation** - Should have reviewed existing code first

## The Path Forward

We're abandoning PR #221 but keeping PR #220. The plan:

1. **Define core types** (Issue #222)
   - ModifierType, ModifierTarget, DamageType
   - No strings, all typed

2. **Combat types** (Issue #223)
   - Universal combat concepts
   - Typed events

3. **Use effects.Core** (Issue #224)
   - Features activate effects
   - Effects modify game state

4. **Effects Manager** (Issue #225)
   - Central management
   - Stacking/suppression rules

5. **Remove magic strings** (Issue #226)
   - Audit and replace all strings
   - Type everything

## The Key Insight

**We need to USE the toolkit, not rebuild it in each module.**

The toolkit should provide the infrastructure that makes implementing game features trivial. When we find ourselves fighting the framework or rebuilding basic concepts, that's a signal we need better infrastructure.

## Combat Is Tricky

We discovered combat is particularly complex:
- Turn order and timing
- Action economy
- Effect interactions
- State management

We need to be slow and deliberate here. Better to have no combat than broken combat.

## The Code We Want to Write

After the infrastructure is in place:
```go
type RageFeature struct {
    *features.Simple
    uses int
}

func (f *RageFeature) Activate(owner Entity) error {
    f.uses--
    return effects.Apply(owner, &RageEffect{
        Duration: 10 * Round,
        Modifiers: []Modifier{
            {core.ModifierAdvantage, dnd5e.TargetStrengthCheck},
            {core.ModifierResistance, combat.DamagePhysical},
        },
        OnTick: f.checkHostileAction,
    })
}
```

~20 lines of actual game logic. That's the goal.

## Final Thought

Sometimes the best code is the code you don't ship. PR #221 taught us what we actually need to build. That makes it valuable, even if we're throwing it away.