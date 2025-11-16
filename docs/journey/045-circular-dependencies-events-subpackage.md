# Journey 045: Solving Circular Dependencies with Events Subpackage

**Date:** 2025-11-16
**Context:** Issue #332 - Character condition tracking implementation

## The Problem

While implementing character condition tracking, we hit a circular dependency:

```
features → dnd5e (for event topics)
dnd5e → conditions (for ConditionType in events)
conditions → dnd5e (for event topics)
```

**The cycle:**
- `features` needs to publish `ConditionAppliedEvent`
- `dnd5e/events.go` defines events and wants to type the `Condition` field
- `conditions` needs event topics to subscribe to combat events
- Circular dependency = compiler error

## Initial Attempts

### Attempt 1: Move ConditionType to conditions package
**Thought:** "Conditions should own ConditionType"

**Problem:** Created the same cycle - `dnd5e` would need to import `conditions` for the event type, but `conditions` imports `dnd5e` for other events.

### Attempt 2: Features create conditions
**Thought:** "Let features create the condition and pass it in the event"

**Problem:** Still circular - `features` imports `conditions`, `conditions` imports `dnd5e`, and `dnd5e` would need to import `conditions` to type the Condition field.

### Attempt 3: Use `any` for Condition field
**Thought:** "Just use runtime type casting"

**Rejected:** This is a shortcut that gives up type safety. We can do better with proper architecture.

## The Root Cause

The issue wasn't about **which package owns what types**. The issue was about **package layering**:

**Key insight:** `dnd5e` package was trying to be BOTH:
1. **Foundational** - defining core types and events
2. **Coordinating** - importing child packages

This violates clean architecture - a package can't be both foundation AND consumer.

## The Solution: Events Subpackage

**Realization:** Event topics/types don't need to live in the root package!

### New Structure
```
dnd5e/
  events/
    events.go          # Event types, topics, ConditionType, ConditionBehavior
  conditions/
    types.go           # imports dnd5e/events
    raging.go          # imports dnd5e/events
  features/
    rage.go            # imports dnd5e/events AND dnd5e/conditions
  character/
    character.go       # imports dnd5e/events AND dnd5e/conditions
  stages.go            # Other dnd5e constants (StageFeatures, etc.)
```

### Dependency Flow (all one-way, NO cycles)
```
dnd5e/events         (foundational - imports nothing from dnd5e)
    ↑
    ├─ dnd5e/conditions
    ├─ dnd5e/features  ← can also import conditions
    └─ dnd5e/character ← can also import conditions & features
```

### What Lives Where

**`dnd5e/events`:**
- Event types (TurnStartEvent, AttackEvent, etc.)
- Event topics (TurnStartTopic, AttackTopic, etc.)
- ConditionType enum and constants
- ConditionBehavior interface

**`dnd5e/conditions`:**
- Condition implementations (RagingCondition, etc.)
- Imports `dnd5e/events` for interface and event topics

**`dnd5e/features`:**
- Feature implementations (Rage, etc.)
- **Creates conditions** and publishes them in events
- Imports `dnd5e/events` for topics AND `dnd5e/conditions` for implementations

**`dnd5e` (root):**
- Game-specific constants that don't fit elsewhere (StageFeatures, etc.)
- Can be imported by conditions/features without creating cycles

## Implementation

### Features Create Conditions

```go
// features/rage.go
func (r *Rage) Activate(ctx context.Context, owner core.Entity, input FeatureInput) error {
    // ... consume resource ...

    // Create the condition with all necessary data
    ragingCondition := &conditions.RagingCondition{
        CharacterID: owner.GetID(),
        DamageBonus: calculateRageDamage(r.level),
        Level:       r.level,
        Source:      r.id,
    }

    // Publish event with the ACTUAL condition (strongly typed!)
    topic := dnd5eEvents.ConditionAppliedTopic.On(input.Bus)
    err := topic.Publish(ctx, dnd5eEvents.ConditionAppliedEvent{
        Target:    owner,
        Type:      dnd5eEvents.ConditionRaging,
        Source:    r.id,
        Condition: ragingCondition,  // ConditionBehavior interface
    })

    return err
}
```

### Character Receives Condition

```go
// character/character.go
func (c *Character) onConditionApplied(ctx context.Context, event dnd5eEvents.ConditionAppliedEvent) error {
    if event.Target.GetID() != c.id {
        return nil
    }

    // Condition is already created! Just apply it
    if err := event.Condition.Apply(ctx, c.bus); err != nil {
        return rpgerr.Wrapf(err, "failed to apply condition")
    }

    c.activeConditions[event.Type] = &ActiveCondition{
        Behavior:  event.Condition,
        Type:      event.Type,
        Source:    event.Source,
        AppliedAt: time.Now(),
    }

    return nil
}
```

## Key Learnings

### 1. Parent→Child Imports Are Fine (With Constraints)

**Question:** "Are parent→child imports bad?"

**Answer:** It depends on what the parent IS:
- **Foundational parent** importing children = BAD (breaks abstraction)
- **Coordinator parent** importing children = FINE (composition)

`dnd5e/events` is foundational - it should not import any dnd5e children.
`dnd5e/features` is a coordinator - it can import `dnd5e/conditions`.

### 2. Circular Dependencies Point to Layering Problems

When you hit circular dependencies, the question isn't "where should this type live?" but rather "what are my architectural layers?"

The solution is usually to **extract a foundational layer** that both packages can depend on.

### 3. Events Subpackage Pattern

This pattern works for ANY complex domain module:

```
mymodule/
  events/        # Foundational - event types, topics, core interfaces
  domain1/       # imports mymodule/events
  domain2/       # imports mymodule/events, can also import domain1
  orchestrator/  # imports everything
```

### 4. Don't Use `any` to Avoid Architecture

Using `any` and runtime type casting is a CODE SMELL that you're avoiding solving the real architectural problem.

If you find yourself thinking "I'll just use `any` here", stop and ask: **"What layering problem am I actually facing?"**

## When to Apply This Pattern

Use an events subpackage when:
1. Multiple domain packages need to communicate via events
2. Events need to reference types from those domain packages
3. Those domain packages also need to use the event system
4. You're hitting circular dependencies

Don't create it prematurely - only when you actually have the cycle.

## Related Patterns

- **Domain Events** - Events as first-class citizens in domain model
- **Hexagonal Architecture** - Clear separation of core vs infrastructure
- **Dependency Inversion** - High-level modules don't depend on low-level modules, both depend on abstractions

## Impact

This architectural decision:
- ✅ Eliminates all circular dependencies
- ✅ Maintains strong typing (no `any` casts)
- ✅ Clear dependency hierarchy
- ✅ Features own the creation logic for their conditions
- ✅ Character is a simple event listener
- ✅ Extensible - adding new conditions doesn't change the pattern

## Quote

> "Parents importing children and children importing parents - are these both generally ok? Does one direction mean you should be more careful?"

Yes! Child→Parent creates impossible cycles. Parent→Child is okay IF the parent is a coordinator, not a foundation. The key is understanding **what role the parent plays** in your architecture.
