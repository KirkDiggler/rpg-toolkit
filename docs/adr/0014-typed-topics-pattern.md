# ADR-0014: Typed Topics Pattern for Event Bus

## Status
Accepted

## Context

The events package had evolved to include three patterns:
1. Legacy ref-based event bus with runtime type checking
2. TypedTopic for pure notifications  
3. ChainedTopic for events with modifier chains

This created unnecessary complexity:
- Events were forced to implement interfaces
- Runtime type assertions everywhere
- Three ways to do the same thing
- Approximately 2000+ lines of code

We needed a simpler, type-safe approach that would serve rulebook authors well.

## Decision

We will implement a typed topics pattern with these key components:

### 1. Events Are Pure Data
```go
// No interface required - just structs
type AttackEvent struct {
    AttackerID string
    TargetID   string
    Damage     int
}
```

### 2. Topics Are Explicitly Defined
```go
// Rulebook defines topics with explicit IDs
var AttackTopic = events.DefineTypedTopic[AttackEvent](Topic("combat.attack"))
var DamageTopic = events.DefineTypedTopic[DamageEvent](Topic("combat.damage"))
```

### 3. The `.On(bus)` Connection Pattern
```go
// Clear, explicit connection to bus
attacks := combat.AttackTopic.On(bus)

// Type-safe subscription
attacks.Subscribe(ctx, func(ctx context.Context, e AttackEvent) error {
    // Handle event
    return nil
})
```

### 4. Two Topic Types for Different Needs

**TypedTopic[T]** for pure notifications:
- Events that notify without transformation
- Simple publish/subscribe pattern

**ChainedTopic[T]** for modifier collection:
- Events that need staged processing
- Collect modifiers into chains
- Execute chains to transform data

### 5. Bus Uses Topic Type
```go
type Topic string  // Local type, not raw string

type EventBus interface {
    Subscribe(topic Topic, handler any) (string, error)
    Unsubscribe(id string) error
    Publish(topic Topic, event any) error
}
```

## Consequences

### Benefits
1. **Type Safety** - Compile-time checking, no runtime assertions
2. **Simple Events** - Just data structs, no interfaces required
3. **Clear API** - The `.On(bus)` pattern is self-documenting
4. **Explicit Topics** - Rulebook owns topic uniqueness
5. **Reduced Complexity** - From 2000+ lines to ~450 lines
6. **IDE-Friendly** - Autocomplete shows all available topics

### Trade-offs
1. **Topic Uniqueness** - Rulebook's responsibility to ensure unique topic IDs
2. **Static Topics** - Topics must be defined at compile time (features remain dynamic)
3. **Explicit IDs** - Must provide topic strings (but only in one place)

### Key Insight
**Features are dynamic, topics are static.** This design separates concerns perfectly:
- Topics are defined statically by the rulebook (compile time)
- Features are loaded dynamically from JSON (runtime)
- Features' Apply() methods explicitly subscribe to known topics
- No dynamic string resolution needed

## Implementation Details

### ChainedEvent Wrapper Solution
For ChainedTopic, we wrap the event and chain together to pass through the bus:
```go
type chainedEvent[T any] struct {
    ctx   context.Context
    event T
    chain chain.Chain[T]
}
```
Handlers unwrap it, modify the chain, and the modified chain is returned to the publisher.

### Shared Data Structures
Events can share common structs without sharing the event type:
```go
// Shared struct
type Damage struct {
    Amount int
    Type   DamageType
}

// Different events using same struct
type AttackEvent struct {
    Damage Damage
}

type SpellEvent struct {
    Damage Damage
}
```

### Testing Strategy
- Testify suite pattern for organization
- SetupTest() for fresh state each test
- Real scenarios (attack damage, conditions)
- 93.5% test coverage achieved

### Migration from Legacy System
1. Remove Event interface requirement
2. Replace ref-based routing with Topic constants
3. Update subscribers to use typed topic APIs
4. Remove legacy bus implementation (~2000 lines → ~500 lines)

## Examples

### Pure Notification
```go
// Define
var LevelUpTopic = events.DefineTypedTopic[LevelUpEvent](Topic("player.levelup"))

// Use
levelups := LevelUpTopic.On(bus)
levelups.Subscribe(ctx, func(ctx context.Context, e LevelUpEvent) error {
    fmt.Printf("Player %s reached level %d\n", e.PlayerID, e.Level)
    return nil
})
```

### Chained Processing
```go
// Define
var AttackChain = events.DefineChainedTopic[AttackEvent](Topic("combat.attack"))

// Use in feature
attacks := AttackChain.On(bus)
attacks.SubscribeWithChain(ctx, func(ctx context.Context, e AttackEvent, chain Chain[AttackEvent]) (Chain[AttackEvent], error) {
    if e.AttackerID == rageOwnerID {
        chain.Add(StageConditions, "rage", rageModifier)
    }
    return chain, nil
})
```

## References
- Journey 007: The Evolution of Typed Topics
- Original events package (pre-refactor)
- Toolkit philosophy: "Pick ONE way", "Optimize for simplicity"