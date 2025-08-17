# Typed Event Bus Architecture: Infrastructure vs Rulebooks

## The Big Picture

The typed event bus is like having **isolated topics** with shared plumbing underneath. Each event type gets its own "channel" with type-safe guarantees, but they all share common infrastructure for concurrency, depth protection, and delivery.

## Infrastructure (RPG Toolkit Provides)

### Core Plumbing - Shared by All Typed Buses
```go
// Common infrastructure all typed buses share
type SharedEventInfra struct {
    publishDepth atomic.Int32    // Cascade protection
    maxDepth     int32           // Config
    metrics      *EventMetrics   // Monitoring
    middleware   []Middleware    // Cross-cutting concerns
}

// Each typed bus gets the shared infra
type TypedBus[T Event] struct {
    *SharedEventInfra             // Embedded shared plumbing
    handlers []Handler[T]         // Type-specific handlers
    mu       sync.RWMutex        // Bus-specific lock
}
```

### What Infrastructure Provides

1. **The Registry** - Central management
```go
// Infrastructure: The registry that manages all typed buses
type EventRegistry struct {
    buses    map[reflect.Type]any  // Type -> TypedBus[T]
    shared   *SharedEventInfra      // Shared plumbing
}

// Get or create a typed bus
func GetBus[T Event](registry *EventRegistry) *TypedBus[T]
```

2. **Common Event Interface**
```go
// Infrastructure defines what an event must have
type Event interface {
    Context() *EventContext  // For modifiers, metadata
}
```

3. **Delivery Guarantees**
- Depth protection (prevent infinite cascades)
- Concurrency safety (lock-free publishing)
- Error propagation
- Metrics/monitoring

4. **Cross-Cutting Middleware**
```go
// Infrastructure can add middleware to all buses
type Middleware func(ctx context.Context, event Event, next Handler) error

// Examples:
// - Logging middleware
// - Metrics middleware  
// - Tracing middleware
```

## Rulebooks (Game Implementations)

### Rulebook Defines Event Types
```go
// dnd5e rulebook defines its events
package dnd5e

// Concrete event types with game-specific data
type DamageDealtEvent struct {
    ctx        *events.EventContext
    Source     EntityID
    Target     EntityID  
    Amount     int
    DamageType string  // "fire", "slashing", etc
}

func (e *DamageDealtEvent) Context() *events.EventContext {
    return e.ctx
}

// Another game event
type SpellCastEvent struct {
    ctx       *events.EventContext
    Caster    EntityID
    SpellName string
    Level     int
    Targets   []EntityID
}
```

### Rulebook Uses Type-Safe Subscriptions
```go
// dnd5e subscribes to its events with full type safety
func SetupCombatHandlers(registry *events.EventRegistry) {
    // Get typed bus for damage events
    damageBus := events.GetBus[*DamageDealtEvent](registry)
    
    // Subscribe with compile-time type safety
    damageBus.Subscribe(func(ctx context.Context, e *DamageDealtEvent) error {
        // e is guaranteed to be *DamageDealtEvent
        if e.DamageType == "fire" && hasFireResistance(e.Target) {
            e.Amount = e.Amount / 2
        }
        return nil
    })
    
    // Get typed bus for spells
    spellBus := events.GetBus[*SpellCastEvent](registry)
    
    spellBus.Subscribe(func(ctx context.Context, e *SpellCastEvent) error {
        // Counterspell logic
        if canCounterspell(e) {
            return ErrSpellCountered
        }
        return nil
    })
}
```

## How It Works - Under the Hood

### 1. Registry Creates Buses Lazily
```go
func GetBus[T Event](registry *EventRegistry) *TypedBus[T] {
    // First call for DamageDealtEvent creates its bus
    // Second call returns the existing bus
    // Each event type gets exactly one bus
    
    key := reflect.TypeOf((*T)(nil))
    if bus, exists := registry.buses[key]; exists {
        return bus.(*TypedBus[T])
    }
    
    // Create new bus with shared infrastructure
    newBus := &TypedBus[T]{
        SharedEventInfra: registry.shared,
        handlers: make([]Handler[T], 0),
    }
    registry.buses[key] = newBus
    return newBus
}
```

### 2. Publishing is Type-Safe and Direct
```go
// No reflection during publish!
func (bus *TypedBus[T]) Publish(event T) error {
    // Check depth (shared infra)
    if !bus.shared.checkDepth() {
        return ErrMaxDepth
    }
    
    // Get handlers (type-safe slice)
    handlers := bus.copyHandlers()
    
    // Direct function calls - no reflection!
    for _, h := range handlers {
        if err := h(ctx, event); err != nil {
            return err
        }
    }
    return nil
}
```

### 3. Topics are Isolated but Share Plumbing
```go
// Each bus is isolated - DamageEvent handlers never see SpellEvents
damageBus := GetBus[*DamageDealtEvent](registry)  // Topic 1
spellBus := GetBus[*SpellCastEvent](registry)     // Topic 2
healBus := GetBus[*HealingEvent](registry)        // Topic 3

// But they all share:
// - Depth protection (if damage triggers spell triggers heal...)
// - Metrics collection
// - Middleware pipeline
// - Error handling patterns
```

## Real-World Usage Example

### Setting Up the System
```go
// Infrastructure setup (done once)
registry := events.NewEventRegistry(
    events.WithMaxDepth(10),
    events.WithMetrics(metricsCollector),
    events.WithMiddleware(loggingMiddleware),
)

// Rulebook registers its handlers
dnd5e.RegisterCombatHandlers(registry)
dnd5e.RegisterMagicHandlers(registry)
dnd5e.RegisterConditionHandlers(registry)
```

### During Gameplay
```go
// Combat system publishes type-safe events
func (c *CombatSystem) DealDamage(source, target EntityID, damage int) {
    damageBus := events.GetBus[*DamageDealtEvent](c.registry)
    
    event := &DamageDealtEvent{
        Source: source,
        Target: target,
        Amount: damage,
        DamageType: "slashing",
    }
    
    // All damage handlers get called with type safety
    err := damageBus.Publish(event)
}
```

### Cross-Module Communication
```go
// Module A publishes
type InventoryChangedEvent struct {
    ctx    *events.EventContext
    Entity EntityID
    Item   ItemID
    Change string // "added", "removed", "equipped"
}

// Module B subscribes with type safety
invBus := events.GetBus[*InventoryChangedEvent](registry)
invBus.Subscribe(func(ctx context.Context, e *InventoryChangedEvent) error {
    if e.Change == "equipped" && isWeapon(e.Item) {
        // Update combat stats
    }
    return nil
})
```

## Benefits of This Architecture

1. **Type Safety Where It Matters**
   - Handlers know exactly what they're getting
   - Compile-time checking
   - IDE autocomplete works

2. **Shared Infrastructure**
   - Don't repeat depth protection, concurrency, etc.
   - Consistent behavior across all event types
   - Central place for cross-cutting concerns

3. **Topic Isolation**
   - Each event type is isolated
   - No accidental cross-talk
   - Clear boundaries

4. **Performance**
   - No reflection in hot path
   - Direct function calls
   - Type checking at compile time

5. **Gradual Adoption**
   - Can start with one event type
   - Other events use old bus
   - Migrate as needed

## The Mental Model

Think of it like **Kafka topics** or **RabbitMQ exchanges**:
- Each event type is a topic
- Handlers subscribe to specific topics
- The infrastructure (registry) is the message broker
- Shared plumbing handles delivery, monitoring, etc.

But with compile-time type safety!

## Summary

**Infrastructure provides:**
- Registry to manage typed buses
- Shared plumbing (depth, concurrency, metrics)
- Event interface and context
- Middleware system

**Rulebooks provide:**
- Concrete event types
- Event handlers with business logic
- Domain-specific event flows

**The Result:**
Type-safe publish/subscribe with isolated topics sharing common infrastructure. Each event type gets its own "channel" but they all use the same underlying delivery mechanisms.