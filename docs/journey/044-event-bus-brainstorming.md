# Journey 044: Event Bus Architecture Brainstorming

*Date: August 2024*  
*Context: Exploring different event bus patterns before implementing TypedRef-based solution*  
*Outcome: Led to TypedRef pattern with context everywhere, rejected reflection*

## Original Exploration

This document captures our initial brainstorming on event bus patterns. We explored industry patterns to understand what works well before designing our own approach.

## How It's Normally Done (Industry Patterns)

### 1. **Topic-Based (Kafka/RabbitMQ Style)**
```go
// Subscribe to topics, not types
bus.Subscribe("combat.*", handler)     // Wildcard subscription
bus.Subscribe("combat.damage", handler) // Specific topic
bus.Publish("combat.damage", event)

// Self-contained: Each subscription is independent
// Can subscribe same handler to multiple topics
```

### 2. **Event Sourcing Pattern (EventStore/Axon)**
```go
// Events are immutable facts in a stream
type EventStream struct {
    aggregateID string
    events      []Event
    version     int
}

// Subscribe to streams or projections
bus.SubscribeToStream("player-123", handler)
bus.SubscribeToCategory("combat", handler)
```

### 3. **Actor Model (Erlang/Akka)**
```go
// Each handler is an actor with a mailbox
type Actor struct {
    mailbox chan Event
}

// Send events to specific actors
actorRef.Send(DamageEvent{})
```

### 4. **Observer Pattern (Classic GoF)**
```go
// Direct registration with subject
damageSubject.Attach(observer)
damageSubject.Notify(event)
```

## Go Idiomatic Approaches

### 1. **Channels All The Way**
```go
// Go loves channels - what if the bus IS channels?
type TypedChan[T Event] chan T

type EventBus struct {
    damage  TypedChan[*DamageEvent]
    turn    TypedChan[*TurnEvent]
    death   TypedChan[*DeathEvent]
}

// Subscribe by reading from channel
go func() {
    for damage := range bus.damage {
        handleDamage(damage)
    }
}()

// Publish by sending
bus.damage <- &DamageEvent{Amount: 10}
```

### 2. **Interface-Based Subscription**
```go
// Go way: small interfaces
type DamageHandler interface {
    HandleDamage(context.Context, *DamageEvent) error
}

type TurnHandler interface {
    HandleTurn(context.Context, *TurnEvent) error  
}

// Handler implements multiple interfaces
type CombatSystem struct{}
func (c *CombatSystem) HandleDamage(...) error { }
func (c *CombatSystem) HandleTurn(...) error { }

// Bus checks interfaces
if dh, ok := handler.(DamageHandler); ok {
    bus.damageHandlers = append(bus.damageHandlers, dh)
}
```

### 3. **Function Types as First-Class**
```go
// Go treats functions as values
type EventBus struct {
    handlers map[reflect.Type][]any
}

// Multiple subscription types
func (bus *EventBus) OnDamage(fn func(*DamageEvent)) {
    bus.handlers[reflect.TypeOf(&DamageEvent{})].append(fn)
}

func (bus *EventBus) OnTurn(fn func(*TurnEvent)) {
    bus.handlers[reflect.TypeOf(&TurnEvent{})].append(fn)
}

// Usage feels Go-like
bus.OnDamage(func(e *DamageEvent) {
    // Handle damage
})
```

## Pragmatic Ideas

### 1. **Multi-Bus with Cross-Registration**
```go
// Each handler can register with multiple typed buses
type CombatLogger struct{}

func (c *CombatLogger) Register(registry *EventRegistry) {
    // Self-contained subscriptions to multiple event types
    GetBus[*DamageEvent](registry).Subscribe(c.logDamage)
    GetBus[*TurnEvent](registry).Subscribe(c.logTurn)
    GetBus[*DeathEvent](registry).Subscribe(c.logDeath)
}
```

### 2. **Tagged Events**
```go
// Events carry tags for routing
type Event interface {
    Tags() []string  // ["combat", "damage", "player"]
}

// Subscribe by tag
bus.SubscribeToTag("combat", handler)  // Gets all combat events
bus.SubscribeToTags([]string{"damage", "player"}, handler)
```

### 3. **Hybrid: Typed Wrappers on Tagged Bus**
```go
// Internal: tag-based routing
// External: type-safe API

func SubscribeDamage(bus *Bus, handler func(*DamageEvent)) {
    bus.SubscribeToTag("damage", func(e Event) {
        if damage, ok := e.(*DamageEvent); ok {
            handler(damage)
        }
    })
}
```

## Hopeful Ideas

### 1. **Code Generation for Type Safety**
```go
//go:generate eventbus-gen

// Generates:
// - Typed subscription methods
// - Typed publish methods  
// - No reflection at runtime

type EventBus = generated.EventBus
```

### 2. **Event Graphs**
```go
// Events know their relationships
type EventGraph struct {
    nodes map[EventType][]EventType
}

// Subscribe to event and its consequences
bus.SubscribeToCascade(DamageEvent, handler)
// Automatically includes: DeathEvent, LootDropEvent, etc.
```

### 3. **Smart Routing with Predicates**
```go
// Rich subscription predicates
bus.Subscribe(
    Where[*DamageEvent](func(e *DamageEvent) bool {
        return e.Amount > 10 && e.Target == "player"
    }),
    handler,
)
```

## Wild Ideas

### 1. **Time-Traveling Event Bus**
```go
// Bus maintains event history
type TemporalBus struct {
    timeline []TimestampedEvent
}

// Subscribe to past events
bus.SubscribeFromTime(time.Now().Add(-5*time.Minute), handler)

// Replay events for debugging
bus.ReplayFrom(checkpoint, handler)
```

### 2. **Quantum Superposition Events**
```go
// Events can be in multiple states until observed
type QuantumEvent struct {
    possibleStates []Event
}

// Collapses when first handler processes it
bus.Publish(QuantumDamage{
    possibleStates: []Event{
        &CriticalHit{Amount: 20},
        &NormalHit{Amount: 10},
        &Miss{},
    },
})
```

### 3. **Neural Event Bus**
```go
// Bus learns patterns and predicts next events
type NeuralBus struct {
    network *NeuralNetwork
}

// Predictive subscription
bus.SubscribeToPredicted(handler) // Called for likely future events

// Bus learns from event patterns
bus.Train(historicalEvents)
```

### 4. **Blockchain Event Bus**
```go
// Every event is a transaction in a chain
type BlockchainBus struct {
    chain []EventBlock
}

// Immutable, verifiable event history
// Can't fake "I killed the dragon" event
```

### 5. **Aspect-Oriented Events**
```go
// Events as join points for aspects
@Before(DamageEvent)
func validateDamage(e *DamageEvent) error { }

@After(DamageEvent)  
func applyDamageEffects(e *DamageEvent) error { }

@Around(CombatEvents)
func combatLogging(proceed func(), e Event) error { }
```

### 6. **Event Combinators**
```go
// Combine events functionally
damageAndDeath := Events.Sequence(DamageEvent, DeathEvent)
damageOrHeal := Events.Either(DamageEvent, HealEvent)
repeatedDamage := Events.Repeat(DamageEvent, 3)

bus.Subscribe(damageAndDeath, handler) // Only called for the sequence
```

## The Self-Contained Subscription Pattern

Yes! Each subscription should be self-contained:

```go
// Each subscription is independent
type Subscription struct {
    id       string
    eventType reflect.Type
    filter   func(Event) bool
    handler  func(context.Context, Event) error
}

// A handler can have multiple self-contained subscriptions
type CombatSystem struct {
    subs []Subscription
}

func (c *CombatSystem) Setup(bus *EventBus) {
    c.subs = append(c.subs, 
        bus.Subscribe(DamageEvent, c.handleDamage),
        bus.Subscribe(TurnEvent, c.handleTurn),
        bus.Subscribe(DeathEvent, c.handleDeath),
    )
}

// Each subscription is isolated
// Unsubscribing one doesn't affect others
// Can be managed independently
```

## Go-Specific Pattern: Registry of Handlers

```go
// Very Go: Register handlers like http.HandleFunc
var DefaultBus = NewEventBus()

func HandleDamage(handler func(*DamageEvent)) {
    DefaultBus.Handle(reflect.TypeOf(&DamageEvent{}), handler)
}

func init() {
    // Self-registration pattern
    HandleDamage(func(e *DamageEvent) {
        // Process damage
    })
    
    HandleTurn(func(e *TurnEvent) {
        // Process turn
    })
}
```

## The Synthesis: Channel-Backed Type-Safe Topics

```go
// Combining Go idioms with type safety
type Topic[T Event] struct {
    name     string
    channel  chan T
    handlers []func(context.Context, T) error
}

type EventBus struct {
    topics sync.Map // Type -> *Topic[T]
}

// Subscribe to typed topic
func Subscribe[T Event](bus *EventBus, handler func(context.Context, T) error) Subscription {
    topic := bus.GetOrCreateTopic[T]()
    return topic.Subscribe(handler)
}

// Each topic runs its own goroutine
func (t *Topic[T]) run() {
    for event := range t.channel {
        // Fan-out to all handlers
        for _, h := range t.handlers {
            go h(context.Background(), event)
        }
    }
}
```

## The Real Question

What problem are we optimizing for?
- **Type safety?** → Typed buses
- **Flexibility?** → Tag-based routing
- **Performance?** → Channel-based
- **Debugging?** → Event sourcing with history
- **Simplicity?** → Basic interface + type assertions
- **Go idioms?** → Channels and interfaces

Maybe the answer is: **Let the use case drive the architecture**