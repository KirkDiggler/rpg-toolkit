# Journey 041: Event Bus Generics Exploration

## Date: 2025-08-13

## The Challenge: Can We Remove Event Type Strings?

Kirk's proposal:
```go
type EventBus interface {
    Publish(event Event) error
    Subscribe[T Event](handler func(T) error) string
}
```

Let's try to implement this...

## Attempt 1: Pure Generics

```go
type Event interface {
    Ref() *core.Ref  // Every event has a ref
}

type Bus struct {
    mu       sync.RWMutex
    handlers map[string][]any  // Key is... what?
}

func (b *Bus) Subscribe[T Event](handler func(T) error) string {
    // Problem: How do we know what event type T is?
    // We need T's type as a string key somehow
    
    // We could use reflection:
    var zero T
    eventType := reflect.TypeOf(zero).String()  // "DamageCalculateEvent"
    
    // But this breaks if T is an interface!
}
```

## Attempt 2: Events Know Their Type

```go
type Event interface {
    Ref() *core.Ref  // Returns something like "combat:event:damage_calculate"
}

type DamageCalculateEvent struct {
    Source     core.Entity
    Target     core.Entity
    BaseDamage string
}

func (e DamageCalculateEvent) Ref() *core.Ref {
    // This is always the same for all instances
    return core.MustParseString("combat:event:damage_calculate")
}

type Bus struct {
    mu       sync.RWMutex
    handlers map[string][]reflect.Value  // Key is ref.String()
}

func (b *Bus) Publish(event Event) error {
    key := event.Ref().String()
    
    // Get handlers for this event type
    handlers := b.handlers[key]
    
    // Call each one via reflection
    for _, h := range handlers {
        h.Call([]reflect.Value{reflect.ValueOf(event)})
    }
    return nil
}

func (b *Bus) Subscribe[T Event](handler func(T) error) string {
    // Still need to know T's ref...
    // We need an instance to get the ref
    var zero T
    key := zero.Ref().String()  // This works if zero value has Ref()
    
    b.handlers[key] = append(b.handlers[key], reflect.ValueOf(handler))
    return generateID()
}
```

## The Problem: Subscribe Needs Type Info

The issue is Subscribe[T] needs to know what ref/type T has, but we only have the type parameter, not an instance.

## Solution: Events as Type Constants

```go
// Each event type has a constant ref
type DamageCalculateEvent struct {
    Source     core.Entity
    Target     core.Entity
}

// Static method or package constant
func (DamageCalculateEvent) EventRef() *core.Ref {
    return core.MustParseString("combat:event:damage_calculate")
}

// Or even simpler - just a method that returns string
func (DamageCalculateEvent) EventType() string {
    return "combat.damage.calculate"
}
```

Then Subscribe could work:

```go
func (b *Bus) Subscribe[T Event](handler func(T) error) string {
    var zero T
    key := zero.EventType()  // Works on zero value!
    
    wrapped := func(e Event) error {
        typed, ok := e.(T)
        if !ok {
            return fmt.Errorf("type mismatch")
        }
        return handler(typed)
    }
    
    b.handlers[key] = append(b.handlers[key], wrapped)
}
```

## The Cleaner Approach: Keep Event Type Explicit

Actually, being explicit might be better:

```go
type EventBus interface {
    Publish(event Event) error
    Subscribe[T Event](eventType string, handler func(T) error) string
}

// Usage is clear
bus.Subscribe[DamageCalculateEvent]("damage.calculate", func(e DamageCalculateEvent) error {
    // Type safe!
})

// Publish uses event's type
event := DamageCalculateEvent{...}
bus.Publish(event)  // Bus gets type from event.Type()
```

## Final Recommendation

```go
type Event interface {
    Type() string  // Or Ref() *core.Ref
}

type EventBus interface {
    Publish(event Event) error
    Subscribe[T Event](eventType string, handler func(T) error) string
    Unsubscribe(id string)
}
```

This gives us:
1. Type safety via generics
2. Explicit event types (no magic)
3. Simple implementation
4. Can't mess up types

The implementation would type-check at runtime but compiler ensures handler signatures match.