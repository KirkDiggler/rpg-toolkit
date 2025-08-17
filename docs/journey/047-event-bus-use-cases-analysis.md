# Journey 047: Event Bus Use Cases Analysis

*Date: August 2024*  
*Context: Analyzed game use cases to inform event bus design*  
*Outcome: Identified need for decoupled mechanics via event-driven pipelines*

## Original Analysis

This analysis explored how events would be used in actual game mechanics, leading to our pipeline-triggering approach.

## Our Actual Need
An internal bus that triggers pipelines for game mechanics, enabling decoupled entity interactions.

## Core Use Cases

### 1. Combat Damage Flow
```go
// Player attacks goblin
// This should trigger a cascade of events/pipelines

// PUBLISH SIDE - Simple and clear
bus.Publish(ctx, &DamageIntentEvent{
    Source: playerID,
    Target: goblinID,
    Amount: 10,
    Type:   "slashing",
})

// This triggers multiple subscribers:
// - Damage calculation pipeline
// - Condition check pipeline  
// - Death check pipeline
// - UI notification pipeline
```

### 2. Condition Application
```go
// Poison is applied to a character
bus.Publish(ctx, &ConditionAppliedEvent{
    Target:    characterID,
    Condition: "poisoned",
    Duration:  3, // turns
    Source:    trapID,
})

// Triggers:
// - Resistance check pipeline
// - Status effect pipeline
// - UI update pipeline
// - Related condition pipeline (poisoned might trigger "sickened")
```

### 3. Turn-Based Events
```go
// Turn starts
bus.Publish(ctx, &TurnStartEvent{
    Entity: characterID,
    Round:  5,
})

// Triggers:
// - Condition tick pipeline (poison damage, healing, etc)
// - Resource regeneration pipeline
// - Buff/debuff expiration pipeline
// - AI decision pipeline (for NPCs)
```

### 4. Environmental Interactions
```go
// Character enters hazardous area
bus.Publish(ctx, &ZoneEnteredEvent{
    Entity: characterID,
    Zone:   "lava_pool",
    Room:   roomID,
})

// Triggers:
// - Environmental damage pipeline
// - Movement validation pipeline
// - Trap activation pipeline
```

## Subscription Patterns

### Pattern A: Specific Event Types (Clean Contract)
```go
// Very clear what you're subscribing to
bus.On(&DamageIntentEvent{}, func(ctx context.Context, e *DamageIntentEvent) error {
    // Process damage intent
    // ctx has game context (room, combat, etc)
    return nil
})

bus.On(&ConditionAppliedEvent{}, func(ctx context.Context, e *ConditionAppliedEvent) error {
    // Check immunities, apply condition
    return nil
})
```

### Pattern B: Topic-Based (More Flexible)
```go
// Subscribe to categories of events
bus.Subscribe("combat.damage.*", func(ctx context.Context, e Event) error {
    // Handle any damage-related event
    switch evt := e.(type) {
    case *DamageIntentEvent:
        // ...
    case *DamageDealtEvent:
        // ...
    }
})

bus.Subscribe("condition.*", func(ctx context.Context, e Event) error {
    // Handle all condition events
})
```

### Pattern C: Pipeline Triggers (What We Really Want?)
```go
// Events trigger pipelines, not direct handlers
bus.TriggersPipeline(&DamageIntentEvent{}, damagePipeline)

// Pipeline processes the event through stages
damagePipeline := Pipeline(
    CalculateBaseDamage,
    ApplyResistances,
    ApplyVulnerabilities,
    ApplyShields,
    DealFinalDamage,
    CheckForDeath,
)
```

## Context vs Event Data Spectrum

### Option 1: Fat Context, Thin Events
```go
// Everything in context
ctx := &GameContext{
    Room:      currentRoom,
    Combat:    activeCombat,
    Entities:  allEntities,
    Source:    attackerID,
    Target:    defenderID,
    Amount:    10,
    DamageType: "fire",
}
bus.Publish(ctx, &GenericDamageEvent{})

// Handlers pull from context
func handleDamage(ctx *GameContext, _ *GenericDamageEvent) error {
    damage := ctx.Amount // Pull from context
}
```

### Option 2: Thin Context, Fat Events (Preferred)
```go
// Context has environment, events have specifics
ctx := &GameContext{
    Room:   currentRoom,
    Combat: activeCombat,
}

event := &DamageIntentEvent{
    Source: attackerID,
    Target: defenderID,
    Amount: 10,
    Type:   "fire",
}
bus.Publish(ctx, event)

// Clean separation
func handleDamage(ctx *GameContext, e *DamageIntentEvent) error {
    // Context has environment
    // Event has specific data
}
```

### Option 3: EventContext Merger (Hybrid)
```go
// Event carries its own context
event := &DamageIntentEvent{
    Context: &EventContext{
        Room:   currentRoom,
        Combat: activeCombat,
    },
    Source: attackerID,
    Target: defenderID,
    Amount: 10,
}
bus.Publish(event)
```

## Decoupling Example: Condition Cascades

This is where events really shine for games:

```go
// Burning causes damage
bus.On(&ConditionTickEvent{Condition: "burning"}, func(ctx context.Context, e *ConditionTickEvent) error {
    // Publish damage event
    return bus.Publish(ctx, &DamageIntentEvent{
        Source: e.Condition,
        Target: e.Entity,
        Amount: 5,
        Type:   "fire",
    })
})

// Damage can cause death
bus.On(&DamageDealtEvent{}, func(ctx context.Context, e *DamageDealtEvent) error {
    if health <= 0 {
        return bus.Publish(ctx, &DeathEvent{Entity: e.Target})
    }
})

// Death triggers loot
bus.On(&DeathEvent{}, func(ctx context.Context, e *DeathEvent) error {
    return bus.Publish(ctx, &LootDropEvent{Source: e.Entity})
})

// Everything is decoupled!
// Conditions don't know about death
// Damage doesn't know about loot
// Death doesn't know what triggers it
```

## My Recommendation

### For Our Needs: Type-Safe Topics with Pipeline Triggers

```go
// 1. Specific event types (clear contracts)
type DamageIntentEvent struct {
    Source EntityID
    Target EntityID
    Amount int
    Type   DamageType
}

// 2. Events implement simple interface
func (e *DamageIntentEvent) Topic() string {
    return "combat.damage.intent"
}

// 3. Subscribe by type OR topic
bus.On(&DamageIntentEvent{}, handler)      // Type-safe
bus.Subscribe("combat.damage.*", handler)   // Flexible

// 4. Events trigger pipelines
bus.OnEvent(&DamageIntentEvent{}, func(ctx context.Context, e *DamageIntentEvent) error {
    return damagePipeline.Process(ctx, e)
})

// 5. Context carries game state
type GameContext struct {
    context.Context
    Room    *Room
    Combat  *Combat
    EventID string  // For correlation
}
```

### Why This Works

1. **Clear contracts** - You know what data each event carries
2. **Decoupled** - Handlers don't know about each other
3. **Pipeline ready** - Events naturally flow into pipelines
4. **Type safe** - No guessing what's in the event
5. **Flexible** - Can subscribe by type or topic pattern

### The Sweet Spot

- **Thin context** (environment/game state)
- **Specific events** (DamageIntent, ConditionApplied, etc)
- **Topic-based routing** (derived from event type)
- **Pipeline triggers** (events start pipelines)

This gives us decoupled entity interactions without over-engineering!