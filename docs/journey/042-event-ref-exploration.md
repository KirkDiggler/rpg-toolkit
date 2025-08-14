# Journey 042: Event Ref Exploration

## Date: 2025-08-13

## The Question

Kirk asked: "What if Event interface had a func that returned *core.Ref?"

This is a great question! Let's explore the implications.

## Current Design

```go
// Event returns a string
type Event interface {
    Type() string
}

// We use TypedRef to tie type and string together
events.Publish(bus, combat.DamageEventRef, &DamageEvent{...})
events.Subscribe(bus, combat.DamageEventRef, handleDamage)
```

## Alternative: Events Know Their Ref

```go
// Event returns its ref
type Event interface {
    EventRef() *core.Ref
}

// Now publish doesn't need the ref!
events.Publish(bus, &DamageEvent{...})

// But subscribe still needs TypedRef for compile-time type safety
events.Subscribe(bus, combat.DamageEventRef, handleDamage)
```

## Pros and Cons

### Current Design (Type() string + TypedRef)

**Pros:**
- Simple Event interface
- Events can be defined anywhere without importing core
- TypedRef ensures publish and subscribe use same string

**Cons:**
- Must pass ref to Publish
- Two sources of truth (event.Type() and ref.String())
- Could mismatch if not careful

### Alternative (EventRef() *core.Ref)

**Pros:**
- Events are self-describing
- Publish doesn't need ref parameter
- Single source of truth for routing
- Can validate at runtime that refs match

**Cons:**
- Every event must import core
- Every event must store or create a ref
- More complex event definition

## The Real Issue

The core tension is between:
1. **Convenience** - Just publish an event without extra parameters
2. **Type Safety** - Ensure publishers and subscribers agree on types
3. **Validation** - Can we detect mismatches?

## Three Approaches

### 1. Trust the Developer (Current)
```go
// Developer ensures event.Type() matches ref.String()
type DamageEvent struct {...}
func (e *DamageEvent) Type() string { return "combat:event:damage" }

var DamageEventRef = &core.TypedRef[*DamageEvent]{
    Ref: core.MustParseString("combat:event:damage"),
}
```

### 2. Events Carry Refs
```go
// Event knows its ref
type DamageEvent struct {...}
func (e *DamageEvent) EventRef() *core.Ref { return damageRef }

// Single ref definition
var damageRef = core.MustParseString("combat:event:damage")
var DamageEventRef = &core.TypedRef[*DamageEvent]{Ref: damageRef}
```

### 3. Code Generation
```go
//go:generate eventgen combat:event:damage
type DamageEvent struct {...}
// Generated: Type(), EventRef(), and TypedRef
```

## My Recommendation

I think Option 2 (Events Carry Refs) is actually cleaner:

```go
package combat

// Single ref definition - source of truth
var damageEventRef = mustParseRef("combat:event:damage")

// Event returns the ref
type DamageEvent struct {
    Source, Target core.Entity
    Amount int
}

func (e *DamageEvent) EventRef() *core.Ref {
    return damageEventRef
}

// TypedRef for subscriptions
var DamageEventRef = &core.TypedRef[*DamageEvent]{
    Ref: damageEventRef,
}

// Usage is cleaner:
events.Publish(bus, &DamageEvent{...})  // No ref needed!
events.Subscribe(bus, DamageEventRef, handler)  // Still type-safe
```

This gives us:
- Single source of truth (the ref variable)
- Clean publish API
- Type-safe subscribe
- Runtime validation possible

## The Decision

Should we refactor to events carrying refs? 

Benefits:
- Cleaner API for publish
- Single source of truth
- Can validate refs match

Costs:
- Breaking change
- Every event needs core import
- Slightly more complex event definition

I think it's worth it for the cleaner API and validation.