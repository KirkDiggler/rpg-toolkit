# Type-Safe Event Bus Design

## Problem Statement

Currently, the event bus uses reflection and runtime type checking for every handler call. What we really want is **type safety** - when you subscribe to a specific event type, you should know at compile-time exactly what data structure you'll receive.

## Core Insight

The ref pointer comparison was trying to enforce "source of truth" but what we actually need is:
- **Type safety**: Subscribe to `DamageEvent`, receive `DamageEvent`
- **Compile-time guarantees**: Wrong types caught during compilation
- **Zero reflection overhead**: Direct function calls

## Design Goals

1. **Complete type safety** - No runtime type assertions needed
2. **Zero reflection** - Direct function calls for performance
3. **Same semantics** - Works like current bus but with types
4. **Easy migration** - Can coexist with current bus

## Proposed Architecture

### Option A: Generic TypedBus per Event Type

```go
// Each event type gets its own bus
damagebus := NewTypedBus[*DamageEvent]()
healBus := NewTypedBus[*HealEvent]()

// Subscribe with full type safety
damagebus.Subscribe(func(ctx context.Context, e *DamageEvent) error {
    // e is guaranteed to be *DamageEvent
    return nil
})
```

**Pros:**
- Maximum type safety
- Zero reflection
- Best performance

**Cons:**
- Need separate bus per event type
- Manual bus management

### Option B: Registry Pattern with Type Keys

```go
// Single registry manages all typed buses
registry := NewTypedEventBus()

// Get or create typed bus for event type
bus := GetBus[*DamageEvent](registry)
bus.Subscribe(handler)

// Or direct subscribe through registry
Subscribe(registry, func(ctx context.Context, e *DamageEvent) error {
    return nil
})
```

**Pros:**
- Single entry point
- Automatic bus creation
- Type safety preserved

**Cons:**
- One lookup to find bus
- Still need type parameter

### Option C: Hybrid with Interface Compatibility

```go
// TypedBus implements EventBus interface
type TypedBus[T Event] struct { ... }

// Can be used as regular EventBus
var bus EventBus = NewTypedBus[*DamageEvent]()

// But also provides typed methods
typedBus := bus.(*TypedBus[*DamageEvent])
typedBus.TypedSubscribe(handler)
```

**Pros:**
- Works with existing code
- Gradual migration possible
- Both typed and untyped APIs

**Cons:**
- Two APIs to maintain
- Some type assertions needed

## Recommended Approach

**Start with Option B (Registry Pattern)** because it:
1. Provides single point of bus management
2. Maintains type safety where it matters (in handlers)
3. Allows gradual migration from current bus
4. Can be wrapped to look like current API

## Implementation Considerations

### 1. Event Type Identity
- Use `reflect.TypeOf()` once during bus creation
- Cache the type-to-bus mapping
- No reflection during publish/subscribe

### 2. Memory Management
- Lazy creation of typed buses
- Buses exist only for event types actually used
- Could add cleanup for unused buses

### 3. Concurrency
- Same lock-free publishing as current bus
- Copy handlers before execution
- No locks during handler calls

### 4. Migration Path

Phase 1: Add typed layer on top
```go
// Existing code still works
bus.Subscribe(ref, handler)

// New typed API available
TypedSubscribe(bus, func(ctx context.Context, e *DamageEvent) error {
    return nil
})
```

Phase 2: Deprecate reflection-based API
- Mark old methods as deprecated
- Provide migration guide
- Tools to help convert

Phase 3: Remove old API
- Clean, type-safe API only
- No reflection overhead
- Better performance

## Usage Examples

### Basic Usage
```go
// Get typed bus from registry
damageBus := GetBus[*DamageEvent](registry)

// Subscribe with full type safety
sub, err := damageBus.Subscribe(func(ctx context.Context, e *DamageEvent) error {
    fmt.Printf("Damage dealt: %d to %s\n", e.Amount, e.Target)
    return nil
})

// Publish - knows the type
err = damageBus.Publish(&DamageEvent{
    Target: "goblin",
    Amount: 10,
})
```

### With Filters
```go
// Type-safe filter
damageBus.SubscribeWithFilter(
    handler,
    func(e *DamageEvent) bool {
        return e.Amount > 5  // Only high damage
    },
)
```

### Cross-Module Events
```go
// Module A defines event
type CombatStarted struct {
    Participants []string
}

// Module B subscribes with type safety
combatBus := GetBus[*CombatStarted](registry)
combatBus.Subscribe(func(ctx context.Context, e *CombatStarted) error {
    // Initialize combat UI
    return nil
})
```

## Benefits

1. **Compile-time safety** - Wrong types caught immediately
2. **Better IDE support** - Autocomplete, refactoring work
3. **Performance** - No reflection overhead
4. **Clarity** - Clear what events a handler processes
5. **Simpler code** - No type assertions in handlers

## Trade-offs

1. **More verbose** - Need type parameters
2. **Multiple buses** - One per event type
3. **Generic constraints** - Requires Go 1.18+

## Decision Points

1. **Should we replace or supplement current bus?**
   - Recommend: Supplement first, migrate gradually

2. **How to handle dynamic event types?**
   - Registry pattern handles this well

3. **What about backward compatibility?**
   - Can provide adapter layer during migration

## Conclusion

A type-safe event bus gives us what we actually want: **when you subscribe to an event type, you know exactly what you'll receive**. This is cleaner, faster, and safer than runtime type checking.

The registry pattern provides the best balance of type safety, usability, and migration path from our current system.