# Events Package - Beautiful Type-Safe Topics

## Philosophy

Events are just data. Topics route them. The `.On(bus)` pattern connects them. ~500 lines total.

## The Magic: `.On(bus)` Pattern

```go
// Define topics at package level - rulebook ensures uniqueness
var AttackTopic = events.DefineTypedTopic[AttackEvent](events.Topic("combat.attack"))

// Connect and use - beautiful!
attacks := AttackTopic.On(bus)
```

This pattern is everything:
- **Explicit** - You see the connection happening
- **Type-safe** - Can't mix types
- **Discoverable** - IDE shows all available topics
- **Clean** - No strings in user code

## Two Patterns for Different Needs

### 1. TypedTopic - Pure Notifications

For events that notify without transformation:

```go
// Define topic constants
const TopicLevelUp events.Topic = "player.levelup"

// Define typed topic using the constant
var LevelUpTopic = events.DefineTypedTopic[LevelUpEvent](TopicLevelUp)

// Event is just data - no interface!
type LevelUpEvent struct {
    PlayerID string
    NewLevel int
}

// Use in features
func OnLevelUp(bus events.EventBus) {
    levelups := LevelUpTopic.On(bus)
    
    levelups.Subscribe(ctx, func(ctx context.Context, e LevelUpEvent) error {
        fmt.Printf("Player %s reached level %d!\n", e.PlayerID, e.NewLevel)
        return nil
    })
}
```

### 2. ChainedTopic - Events with Modifier Chains

For events that need staged modifier processing:

```go
// Define topic constant
const TopicAttack events.Topic = "combat.attack"

// Define chained topic using the constant
var AttackChain = events.DefineChainedTopic[AttackEvent](TopicAttack)

// Event is still just data
type AttackEvent struct {
    AttackerID string
    TargetID   string
    Damage     int
}

// Features add modifiers
func (r *Rage) Apply(bus events.EventBus) error {
    attacks := AttackChain.On(bus)
    
    attacks.SubscribeWithChain(ctx, func(ctx context.Context, e AttackEvent, chain chain.Chain[AttackEvent]) (chain.Chain[AttackEvent], error) {
        if e.AttackerID == r.ownerID {
            chain.Add(combat.StageConditions, "rage", func(ctx context.Context, e AttackEvent) (AttackEvent, error) {
                e.Damage += 2
                return e, nil
            })
        }
        return chain, nil
    })
}

// Combat system uses it
attack := AttackEvent{AttackerID: "barbarian", Damage: 10}
chain := events.NewStagedChain[AttackEvent](stages)

attacks := AttackChain.On(bus)
modifiedChain, _ := attacks.PublishWithChain(ctx, attack, chain)
result, _ := modifiedChain.Execute(ctx, attack)
// result.Damage is now 12
```

## Key Design Wins

1. **No Event Interface** - Events are pure data structs
2. **No Strings in User Code** - Topics defined once with explicit IDs
3. **The `.On(bus)` Pattern** - Makes connections explicit and beautiful
4. **Type Safety Without Ceremony** - Generics do the work
5. **Features Are Dynamic, Topics Are Static** - Perfect separation

## Complete Example: Rulebook Events

```go
package combat

import (
    "context"
    "github.com/KirkDiggler/rpg-toolkit/events"
)

// Define topic constants - explicit and reusable
const (
    TopicCombatStart events.Topic = "combat.start"
    TopicCombatEnd   events.Topic = "combat.end"
    TopicAttack      events.Topic = "combat.attack"
    TopicDamage      events.Topic = "combat.damage"
    TopicSave        events.Topic = "combat.save"
)

// Define stage constants - explicit processing order
const (
    StageBase       chain.Stage = "base"
    StageFeatures   chain.Stage = "features"
    StageConditions chain.Stage = "conditions"
    StageEquipment  chain.Stage = "equipment"
    StageFinal      chain.Stage = "final"
)

// Define typed topics using the constants
var (
    // Pure notifications
    CombatStartTopic = events.DefineTypedTopic[CombatStartEvent](TopicCombatStart)
    CombatEndTopic   = events.DefineTypedTopic[CombatEndEvent](TopicCombatEnd)
    
    // Chained events for modifiers
    AttackChain = events.DefineChainedTopic[AttackEvent](TopicAttack)
    DamageChain = events.DefineChainedTopic[DamageEvent](TopicDamage)
    SaveChain   = events.DefineChainedTopic[SaveEvent](TopicSave)
)

// Events are just data
type AttackEvent struct {
    AttackerID string
    TargetID   string
    Damage     int
    Critical   bool
}

type DamageEvent struct {
    SourceID   string
    TargetID   string
    Amount     int
    DamageType string
}

// Shared structs for common data
type Damage struct {
    Amount int
    Type   string
}

// Features use the topics
type BlessFeature struct {
    targetID string
    bonus    int
}

func (b *BlessFeature) Apply(bus events.EventBus) error {
    // Beautiful connection pattern
    attacks := AttackChain.On(bus)
    saves := SaveChain.On(bus)
    
    // Type-safe subscriptions
    attacks.SubscribeWithChain(ctx, b.modifyAttack)
    saves.SubscribeWithChain(ctx, b.modifySave)
    
    return nil
}
```

## API Reference

### Defining Topics

```go
// Define constants first - explicit and reusable
const (
    TopicMyEvent  events.Topic = "my.event"
    TopicMyAction events.Topic = "my.action"
)

// For pure notifications
var MyTopic = events.DefineTypedTopic[MyEvent](TopicMyEvent)

// For events with chains
var MyChain = events.DefineChainedTopic[MyAction](TopicMyAction)
```

### Using Topics

```go
// Connect to bus
topic := MyTopic.On(bus)

// Subscribe
id, err := topic.Subscribe(ctx, handler)

// Publish
err := topic.Publish(ctx, event)

// Unsubscribe
err := topic.Unsubscribe(ctx, id)
```

### Using Chains

```go
// Connect to bus
chain := MyChain.On(bus)

// Subscribe with chain
id, err := chain.SubscribeWithChain(ctx, handler)

// Publish with chain
modifiedChain, err := chain.PublishWithChain(ctx, event, chain)

// Execute chain
result, err := modifiedChain.Execute(ctx, event)
```

## File Structure

```
events/
├── bus.go           # Simple EventBus implementation (~100 lines)
├── topic.go         # Topic type definition (9 lines)
├── topic_def.go     # Topic definitions with .On(bus) (~50 lines)
├── typed_topic.go   # TypedTopic implementation (~70 lines)
├── chained_topic.go # ChainedTopic implementation (~130 lines)
├── chain.go         # StagedChain implementation (~120 lines)
├── errors.go        # Error definitions (~25 lines)
└── README.md        # This file
```

## Design Philosophy

This package embodies the toolkit principles:

1. **Pick ONE Way** - Two clear patterns, each for specific needs
2. **Optimize for Simplicity** - No unnecessary abstractions
3. **Make It Impossible to Use Wrong** - Type safety prevents errors
4. **Explicit Over Implicit** - The `.On(bus)` pattern shows intent
5. **Data Over Behavior** - Events are just structs

## For Rulebook Authors

Your workflow is simple:

1. **Define your topics** with explicit IDs
2. **Define your events** as plain structs
3. **Connect with `.On(bus)`** in your features
4. **Subscribe with type safety**
5. **Publish without fear**

No strings to typo. No interfaces to implement. No runtime type assertions.

Just beautiful, type-safe event routing.