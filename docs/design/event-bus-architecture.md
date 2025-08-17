# Event Bus Architecture

**Status: IMPLEMENTED - This is the authoritative design for the RPG Toolkit event bus**

*See also: [typed-bus-architecture.md](typed-bus-architecture.md) for infrastructure/rulebook split*

## Core Concept
An internal event bus that uses TypedRefs for type-safe subscriptions and triggers pipelines for game mechanics.

## The TypedRef Pattern (Already Built)

```go
// We already have this in core!
type TypedRef[T any] struct {
    Ref *Ref
}

// Example typed refs for events
var DamageIntentRef = &core.TypedRef[*DamageIntentEvent]{
    Ref: core.MustParseString("combat:damage:intent"),
}

var ConditionAppliedRef = &core.TypedRef[*ConditionAppliedEvent]{
    Ref: core.MustParseString("condition:applied"),
}
```

## Event Definition Pattern

```go
// Events are simple structs with data
type DamageIntentEvent struct {
    Source EntityID
    Target EntityID
    Amount int
    Type   DamageType
}

// Events know their ref (for routing)
func (e *DamageIntentEvent) EventRef() *core.Ref {
    return DamageIntentRef.Ref
}

// Events carry context
func (e *DamageIntentEvent) Context() *EventContext {
    return e.ctx // Set during creation
}
```

## Subscription API

### Type-Safe Subscription Using TypedRef
```go
// Primary API - uses TypedRef for compile-time safety
bus.Subscribe(DamageIntentRef, func(ctx context.Context, e *DamageIntentEvent) error {
    // e is guaranteed to be *DamageIntentEvent
    // No type assertions needed!
    return nil
})

// The Subscribe function signature (already exists in typed.go!)
func Subscribe[T Event](
    bus EventBus,
    ref *core.TypedRef[T],
    handler func(context.Context, T) error,
    opts ...Option[T],
) (string, error)
```

### Pattern-Based Subscription (Secondary API)
```go
// For cross-cutting concerns like logging
bus.SubscribePattern("combat:*", func(ctx context.Context, e Event) error {
    // Handle all combat events
    log.Printf("Combat event: %v", e.EventRef())
    return nil
})

// Or with type checking
bus.SubscribePattern("condition:*", func(ctx context.Context, e Event) error {
    switch evt := e.(type) {
    case *ConditionAppliedEvent:
        // Handle application
    case *ConditionRemovedEvent:
        // Handle removal
    }
    return nil
})
```

## Publishing API

### Type-Safe Publishing
```go
// Publish with automatic ref detection
event := &DamageIntentEvent{
    Source: playerID,
    Target: goblinID,
    Amount: 10,
    Type:   "slashing",
}

// Event knows its ref, so just publish
err := bus.Publish(ctx, event)
```

### With Context Enhancement
```go
// Game context flows through
gameCtx := &GameContext{
    Room:    currentRoom,
    Combat:  activeCombat,
    Round:   5,
}

// Publish with game context
err := bus.PublishWithContext(gameCtx, event)
```

## Pipeline Integration

### Events Trigger Pipelines
```go
// Define a damage pipeline
damagePipeline := pipeline.New(
    pipeline.Stage("validate", ValidateDamage),
    pipeline.Stage("calculate", CalculateBaseDamage),
    pipeline.Stage("resistance", ApplyResistances),
    pipeline.Stage("shields", ApplyShields),
    pipeline.Stage("apply", ApplyFinalDamage),
    pipeline.Stage("check_death", CheckForDeath),
)

// Connect event to pipeline
bus.Subscribe(DamageIntentRef, func(ctx context.Context, e *DamageIntentEvent) error {
    result, err := damagePipeline.Process(ctx, e)
    if err != nil {
        return err
    }
    
    // Pipeline might produce new events
    if death := result.Get("death_event"); death != nil {
        return bus.Publish(ctx, death.(*DeathEvent))
    }
    return nil
})
```

### Or Direct Pipeline Registration
```go
// Shorthand for event → pipeline
bus.TriggersPipeline(DamageIntentRef, damagePipeline)

// Internally does the subscription and result handling
```

## Complete Usage Example

```go
// 1. Define your events with refs
var DamageIntentRef = &core.TypedRef[*DamageIntentEvent]{
    Ref: core.MustParseString("combat:damage:intent"),
}

type DamageIntentEvent struct {
    ctx    *EventContext
    Source EntityID
    Target EntityID
    Amount int
    Type   DamageType
}

func (e *DamageIntentEvent) EventRef() *core.Ref {
    return DamageIntentRef.Ref
}

func (e *DamageIntentEvent) Context() *EventContext {
    return e.ctx
}

// 2. Set up handlers
func SetupCombatHandlers(bus EventBus) {
    // Type-safe damage handler
    bus.Subscribe(DamageIntentRef, func(ctx context.Context, e *DamageIntentEvent) error {
        // Process damage with full type safety
        target := GetEntity(e.Target)
        target.TakeDamage(e.Amount, e.Type)
        
        // Might trigger other events
        if target.Health <= 0 {
            return bus.Publish(ctx, &DeathEvent{Entity: e.Target})
        }
        return nil
    })
    
    // Condition handler
    bus.Subscribe(ConditionAppliedRef, func(ctx context.Context, e *ConditionAppliedEvent) error {
        entity := GetEntity(e.Target)
        
        // Check immunity
        if entity.IsImmuneTo(e.Condition) {
            return nil
        }
        
        // Apply condition
        entity.AddCondition(e.Condition, e.Duration)
        return nil
    })
    
    // Cross-cutting logger
    bus.SubscribePattern("combat:*", LogCombatEvent)
}

// 3. During gameplay
func PlayerAttack(bus EventBus, attacker, defender EntityID) error {
    // Create event with context
    event := &DamageIntentEvent{
        ctx:    NewEventContext(),
        Source: attacker,
        Target: defender,
        Amount: RollDamage(),
        Type:   "slashing",
    }
    
    // Publish triggers all handlers
    return bus.Publish(context.Background(), event)
}
```

## Context Strategy

```go
// EventContext for game-specific data
type EventContext struct {
    // Event metadata
    EventID   string
    Timestamp time.Time
    
    // Game context
    Room      *Room
    Combat    *Combat
    
    // Modifiers that affect this event
    Modifiers []Modifier
    
    // Pipeline working data
    Pipeline  map[string]any
}

// Merge with Go's context.Context
type GameContext struct {
    context.Context        // For cancellation, timeouts
    *EventContext         // For game data
}
```

## Implementation Plan

### Phase 1: Core Bus with TypedRefs
- [x] TypedRef already exists in core
- [x] Basic bus exists
- [ ] Update Subscribe to use TypedRef properly
- [ ] Ensure ref matching works by value

### Phase 2: Pattern Subscriptions
- [ ] Add SubscribePattern for wildcards
- [ ] Implement pattern matching on refs

### Phase 3: Pipeline Integration
- [ ] Create pipeline package
- [ ] Add TriggersPipeline helper
- [ ] Handle pipeline results → new events

### Phase 4: Game Events
- [ ] Define core game events (damage, condition, death, etc)
- [ ] Create TypedRefs for each
- [ ] Wire up handlers

## Benefits

1. **Type Safety** - TypedRef ensures compile-time type checking
2. **No Reflection in Hot Path** - Direct handler calls
3. **Decoupled** - Events enable loose coupling between systems
4. **Pipeline Ready** - Events naturally flow into processing pipelines
5. **Pattern Matching** - Can subscribe to event categories
6. **Already Partially Built** - We have TypedRef and basic bus!

## Summary

This design:
- Uses our existing TypedRef for type-safe subscriptions
- Keeps events as simple data carriers
- Routes by ref values (which we just fixed!)
- Triggers pipelines for complex processing
- Enables the decoupled entity interactions we want

The key insight: **TypedRef gives us type safety, refs give us routing, pipelines give us processing**