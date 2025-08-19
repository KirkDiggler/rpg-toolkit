# Chain Pattern Documentation

## Core Concept

Events are immutable data. Chains collect modifiers. Topics route events to subscribers.

## Two Types of Events

### 1. Pure Notifications
Events that notify without transformation. Use `Topic[T]`.

Examples:
- `ConditionApplyEvent` - Notify that a condition was applied
- `LevelUpEvent` - Notify that a character leveled up
- `RestCompletedEvent` - Notify that a rest completed

### 2. Chained Processing  
Events that need modifier collection and transformation. Use `ChainedTopic[T]`.

Examples:
- `AttackEvent` - Collect damage modifiers
- `DamageEvent` - Collect resistance/vulnerability
- `SaveEvent` - Collect save bonuses

## The Chain Pattern

```go
// 1. Create event (immutable data)
attack := AttackEvent{
    AttackerID: "barbarian-123", 
    TargetID:   "goblin-456",
    Damage:     10,
}

// 2. Create chain for THIS execution
chain := NewStagedChain[AttackEvent](stages)

// 3. Publish to collect modifiers (chain is built up)
attacks := GetChainedTopic[AttackEvent](bus, "combat.attack")
modifiedChain, _ := attacks.PublishWithChain(ctx, attack, chain)

// 4. Execute chain to transform event
result, _ := modifiedChain.Execute(ctx, attack)
// result.Damage is now modified (e.g., 10 + 2 rage + 4 bless = 16)
```

## How Conditions Work

Conditions have an owner and check if events apply to them:

```go
type RageCondition struct {
    ownerID string // Who has this condition
}

func (r *RageCondition) Apply(bus *EventBus) error {
    attacks := GetChainedTopic[AttackEvent](bus, "combat.attack")
    
    attacks.SubscribeWithChain(ctx, func(ctx context.Context, event AttackEvent, chain chain.Chain[AttackEvent]) (chain.Chain[AttackEvent], error) {
        // Check: Is this MY attack?
        if event.AttackerID == r.ownerID {
            // Yes - add my modifier
            chain.Add(StageConditions, "rage-"+r.ownerID, func(ctx context.Context, e AttackEvent) (AttackEvent, error) {
                e.Damage += 2
                return e, nil
            })
        }
        // Return the chain (modified or not)
        return chain, nil
    })
}
```

## Critical Design Rules

### Rule 1: Events Don't Embed Chains
```go
// ❌ WRONG
type AttackEvent struct {
    Chain chain.Chain[AttackEvent] // NO!
}

// ✅ RIGHT
type AttackEvent struct {
    AttackerID string
    TargetID   string
    Damage     int
    // Just data, no chain
}
```

### Rule 2: Chains Are Created Per Execution
```go
// Each attack gets its own chain
attack1Chain := NewStagedChain[AttackEvent](stages)
result1 := ProcessAttack(attack1, attack1Chain)

attack2Chain := NewStagedChain[AttackEvent](stages)
result2 := ProcessAttack(attack2, attack2Chain)
```

### Rule 3: Conditions Check Ownership
```go
// Rage only modifies if owner is attacking
if event.AttackerID == rage.ownerID {
    chain.Add(...)
}

// Armor only modifies if owner is defending
if event.TargetID == armor.ownerID {
    chain.Add(...)
}
```

### Rule 4: PublishWithChain Returns the Chain
```go
// Caller has the event, needs the modified chain back
modifiedChain, _ := topic.PublishWithChain(ctx, event, chain)
// NOT: modifiedEvent, _ := topic.PublishWithChain(...)
```

## Why This Pattern?

1. **Events are immutable** - Just data about what's happening
2. **Chains are transient** - Created, built up, executed, discarded
3. **Conditions are decoupled** - Subscribe and add modifiers when relevant
4. **Execution is explicit** - Publish collects, Execute transforms

## Common Mistakes

### Mistake 1: Trying to Return Modified Events from Publish
```go
// ❌ WRONG - Publish doesn't transform events
modifiedEvent := topic.Publish(ctx, event)

// ✅ RIGHT - Publish collects modifiers into chain
modifiedChain := topic.PublishWithChain(ctx, event, chain)
result := modifiedChain.Execute(ctx, event)
```

### Mistake 2: Mutating Events in Handlers
```go
// ❌ WRONG - Don't mutate the original
handler(event AttackEvent) {
    event.Damage += 2 // Mutating!
}

// ✅ RIGHT - Return transformed copy
modifier(ctx, event AttackEvent) (AttackEvent, error) {
    event.Damage += 2 // Local copy
    return event, nil
}
```

### Mistake 3: Not Checking Event Relevance
```go
// ❌ WRONG - Modifies ALL attacks
chain.Add(StageConditions, "rage", rageModifier)

// ✅ RIGHT - Only modifies OUR attacks
if event.AttackerID == rage.ownerID {
    chain.Add(StageConditions, "rage", rageModifier)
}
```

## Summary

- Events = Immutable data
- Chains = Modifier collectors
- Topics = Event routers
- Conditions = Subscribers that add modifiers when relevant

The chain is built during publish, executed separately, and discarded after use.