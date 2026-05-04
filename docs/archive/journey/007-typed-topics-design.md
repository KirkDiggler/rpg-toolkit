# Journey 007: The Evolution of Typed Topics

## The Starting Point

We began with a prototyping session to create a strongly typed event bus. The initial implementation had grown to include:
- Legacy ref-based event bus with runtime type checking
- TypedTopic for pure notifications
- ChainedTopic for events with modifier chains
- Events implementing an Event interface

But stepping back, we realized we had THREE patterns when we only needed two, and events were forced to implement interfaces when they should just be data.

## The Revelation

Kirk asked the key question: "Should events declare their topic?" This led to a critical brainstorming session about how to couple events with topics while preventing foot-guns.

## The Design Evolution

### First Insight: Who Are Our Users?

We identified three layers with different needs:
- **Toolkit**: Type safety, simple contracts, flexibility
- **Rulebooks**: Data-driven, modular, clear activation patterns  
- **Game Server**: Orchestration, content building, world management

The rulebook is our primary user - they need explicit, self-documenting patterns.

### The Breakthrough: `.On(bus)` Pattern

Kirk's reaction said it all: "you had me at `attacks := combat.AttackTopic.On(bus)` 🔥"

This pattern is beautiful because:
- Reads like English
- Makes the connection explicit
- No strings in user code
- Type safety without ceremony

### The Final Design

```go
// In rulebook - no strings visible to users!
package combat

// Define typed topics
var (
    AttackTopic = events.DefineTypedTopic[AttackEvent]()
    DamageTopic = events.DefineTypedTopic[DamageEvent]() 
    SaveTopic   = events.DefineTypedTopic[SaveEvent]()
)

// Events are just data - no interface needed!
type AttackEvent struct {
    AttackerID string
    TargetID   string
    Damage     int
}

// Usage in features/conditions
func (r *Rage) Apply(bus events.EventBus) error {
    attacks := combat.AttackTopic.On(bus)
    
    attacks.Subscribe(ctx, func(ctx context.Context, e AttackEvent) error {
        if e.AttackerID == r.ownerID {
            // Add rage bonus
        }
        return nil
    })
}
```

For chained events:
```go
var AttackChain = events.DefineChainedTopic[AttackEvent]()

func (r *Rage) Apply(bus events.EventBus) error {
    attacks := combat.AttackChain.On(bus)
    
    attacks.SubscribeWithChain(ctx, func(ctx context.Context, e AttackEvent, chain Chain[AttackEvent]) (Chain[AttackEvent], error) {
        if e.AttackerID == r.ownerID {
            chain.Add(StageConditions, "rage", rageModifier)
        }
        return chain, nil
    })
}
```

## Why This Design Wins

1. **No Event interface** - Events are pure data structs
2. **No strings in user code** - Topic identity is hidden
3. **Discovery is trivial** - IDE autocomplete shows all topics
4. **Type safety without ceremony** - Can't mix up types at compile time
5. **The `.On(bus)` pattern** - Clear, explicit, beautiful

## Hidden Dragons We Identified (And How Kirk Slayed Them)

1. **Topic uniqueness** - Without strings, how do we ensure topics don't collide?
   - **SOLVED**: Rulebook's responsibility - they provide explicit Topic IDs
   - `DefineTypedTopic[AttackEvent](Topic("combat.attack"))`

2. **Cross-package event sharing** - Who owns shared event types?
   - **SOLVED**: Share structs, not events. Each package defines its own events using common structs
   - `type Damage struct` can be embedded in both `AttackEvent` and `SpellEvent`

3. **Dynamic subscription patterns** - Data-driven features loaded from JSON
   - **SOLVED**: Features are dynamic, topics are static!
   - Features loaded from JSON have explicit Apply() methods with typed subscriptions
   - No dynamic string resolution needed

4. **Event evolution** - Versioning events over time
   - Solution: New topics for new versions during migration

5. **Chain registry** - How to pass chains between publish and subscribe without strings
   - Solution: Use execution IDs or pointer identity

## The Key Insight

**Features are dynamic, topics are static!** This resolves the impedance mismatch:
- Topics are defined at compile time by the rulebook
- Features are loaded at runtime from JSON
- But each feature's Apply() method explicitly subscribes to known topics
- Perfect blend of static type safety and dynamic configuration

The Bus takes `Topic` type (not raw strings), adding another layer of type safety.

## Decision: Build It

Despite the dragons, this design aligns perfectly with toolkit philosophy:
- **Simple** - No unnecessary abstractions
- **Explicit** - Clear what's happening  
- **Type-safe** - Can't mess it up
- **Flexible** - Rulebooks define what they need

The `.On(bus)` pattern alone makes this worth pursuing. It's the kind of API that makes people smile.

## Implementation Discoveries

### The Chain Registry Problem
Initially tried to use a separate registry for chain handlers, but this created hidden global state. The solution was elegant: wrap the event and chain together in a `chainedEvent` struct and pass it through the regular bus. Handlers unwrap it, modify the chain, and pass it back.

### Type Constants Pattern
Instead of raw strings everywhere, we use typed constants:
- `events.Topic` for topic IDs
- `chain.Stage` for processing stages

The rulebook defines constants, ensuring no magic strings in usage.

### The Bus IS the Registry
Key insight: The bus already stores handlers - we don't need another registry. For ChainedTopic, we store the actual chain handlers in the bus and retrieve them during PublishWithChain.

## What We Actually Built

1. ✅ Removed the Event interface completely
2. ✅ Implemented `DefineTypedTopic[T]()` and `DefineChainedTopic[T]()`  
3. ✅ Added the `.On(bus)` method to topic definitions
4. ✅ Used explicit Topic constants instead of hiding strings
5. ✅ Created comprehensive test suites with 93.5% coverage

## Final Implementation Stats

- **~500 lines of production code**
- **~450 lines of test code**
- **93.5% test coverage**
- **Zero interfaces required for events**
- **Two clear patterns: TypedTopic and ChainedTopic**

## Lessons Learned

1. **Step back before committing code** - Brainstorming mode reveals better designs
2. **Consider the user first** - Rulebook authors need clarity, not flexibility
3. **Question every interface** - The Event interface was unnecessary baggage
4. **Beautiful APIs matter** - `.On(bus)` makes people want to use it
5. **Perfect is the enemy of good** - Dragons exist, but the 80% case is worth optimizing
6. **The simplest solution often emerges** - ChainedEvent wrapper was cleaner than a registry
7. **Test suites prove the design** - 93.5% coverage validates our approach

## Final Reflection

This design session exemplifies the toolkit philosophy: pick ONE way, optimize for simplicity, and make it impossible to use wrong.

The `.On(bus)` pattern alone justifies this entire refactor. It's the kind of API that makes developers smile when they use it. Combined with typed constants for Topics and Stages, we've eliminated magic strings while keeping the flexibility rulebooks need.

From 2000+ lines of complex event system to ~500 lines of focused, type-safe beauty. This is what happens when you question every assumption and focus on the user's needs.