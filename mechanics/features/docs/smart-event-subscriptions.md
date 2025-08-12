# Smart Event Subscriptions - Filter at the Bus Level

## The Problem with Current Subscriptions

```go
// Current: Every handler checks if event is relevant
f.Subscribe(bus, events.EventBeforeTakeDamage, 50,
    func(ctx context.Context, e events.Event) error {
        // EVERY handler does this check
        if e.Target().GetID() != f.Owner().GetID() {
            return nil  // Wasted function call
        }
        // ... actual logic
    })
```

## Enhanced Subscription with Filters

```go
// events/subscription.go
type SubscriptionFilter struct {
    SourceID  string  // Only events from this source
    TargetID  string  // Only events targeting this
    RoomID    string  // Only events in this room
    Metadata  map[string]interface{}  // Match context values
}

// Subscribe with filter
func (bus *EventBus) SubscribeFiltered(
    eventType string,
    filter SubscriptionFilter,
    priority int,
    handler HandlerFunc,
) SubscriptionID {
    // Bus only calls handler if filter matches
    return bus.subscribeInternal(eventType, priority, func(ctx context.Context, e Event) error {
        // Check filters BEFORE calling handler
        if filter.SourceID != "" && e.Source().GetID() != filter.SourceID {
            return nil  // Skip
        }
        if filter.TargetID != "" && e.Target().GetID() != filter.TargetID {
            return nil  // Skip
        }
        // All filters passed, call handler
        return handler(ctx, e)
    })
}
```

## Features Use Smart Subscriptions

```go
// Rage only subscribes to damage targeting SELF
func (r *RageFeature) apply(f *features.SimpleFeature, bus events.EventBus) error {
    // Only get called for damage to ME
    f.SubscribeFiltered(bus, events.EventBeforeTakeDamage,
        events.SubscriptionFilter{
            TargetID: f.Owner().GetID(),  // Only MY damage
        },
        50,
        func(ctx context.Context, e events.Event) error {
            // No need to check target - bus already filtered!
            if !r.isActive {
                return nil
            }
            
            // Apply resistance logic
            damageType, _ := e.Context().Get("damage_type")
            // ...
        })
    
    // Only get called for MY attacks
    f.SubscribeFiltered(bus, events.EventOnDamageRoll,
        events.SubscriptionFilter{
            SourceID: f.Owner().GetID(),  // Only MY attacks
        },
        50,
        func(ctx context.Context, e events.Event) error {
            // No need to check source!
            if !r.isActive {
                return nil
            }
            
            // Add rage damage
            // ...
        })
    
    return nil
}
```

## Even Better: Fluent Subscription Builder

```go
// Fluent API for complex filters
type SubscriptionBuilder struct {
    bus      *EventBus
    event    string
    priority int
    filters  SubscriptionFilter
}

func (bus *EventBus) On(eventType string) *SubscriptionBuilder {
    return &SubscriptionBuilder{
        bus:   bus,
        event: eventType,
    }
}

func (s *SubscriptionBuilder) FromSource(id string) *SubscriptionBuilder {
    s.filters.SourceID = id
    return s
}

func (s *SubscriptionBuilder) ToTarget(id string) *SubscriptionBuilder {
    s.filters.TargetID = id
    return s
}

func (s *SubscriptionBuilder) InRoom(id string) *SubscriptionBuilder {
    s.filters.RoomID = id
    return s
}

func (s *SubscriptionBuilder) WithPriority(p int) *SubscriptionBuilder {
    s.priority = p
    return s
}

func (s *SubscriptionBuilder) Do(handler HandlerFunc) SubscriptionID {
    return s.bus.SubscribeFiltered(s.event, s.filters, s.priority, handler)
}
```

## Clean Feature Code

```go
func (r *RageFeature) apply(f *features.SimpleFeature, bus events.EventBus) error {
    ownerID := f.Owner().GetID()
    
    // Super clean subscription!
    bus.On(events.EventBeforeTakeDamage).
        ToTarget(ownerID).
        WithPriority(50).
        Do(func(ctx context.Context, e events.Event) error {
            if !r.isActive {
                return nil
            }
            // Apply resistance
            return r.applyDamageResistance(e)
        })
    
    bus.On(events.EventOnDamageRoll).
        FromSource(ownerID).
        WithPriority(50).
        Do(func(ctx context.Context, e events.Event) error {
            if !r.isActive {
                return nil
            }
            // Add damage bonus
            return r.applyDamageBonus(e)
        })
    
    return nil
}

// Sneak Attack with target
func (s *SneakAttackFeature) applyToTarget(target core.Entity, bus events.EventBus) error {
    bus.On(events.EventOnDamageRoll).
        FromSource(s.owner.GetID()).
        ToTarget(target.GetID()).  // Only damage to THIS target
        WithPriority(50).
        Do(func(ctx context.Context, e events.Event) error {
            if s.usedThisTurn {
                return nil
            }
            // Add sneak attack damage
            return s.applySneakDamage(e)
        })
    
    return nil
}
```

## Performance Benefits

```go
// Before: 100 features × 10 events/turn = 1000 handler calls
// Most return early after checking relevance

// After: Only relevant handlers called
// If event targets "goblin-1", only handlers subscribed to "goblin-1" are called
```

## The EventBus Could Use Maps for O(1) Filtering

```go
type EventBus struct {
    // Subscriptions indexed by filter criteria
    byTarget map[string][]subscription  // Quick lookup by target
    bySource map[string][]subscription  // Quick lookup by source
    byRoom   map[string][]subscription  // Quick lookup by room
    global   []subscription             // No filter
}

func (bus *EventBus) Publish(ctx context.Context, e Event) error {
    var handlers []subscription
    
    // Collect relevant handlers
    if e.Target() != nil {
        handlers = append(handlers, bus.byTarget[e.Target().GetID()]...)
    }
    if e.Source() != nil {
        handlers = append(handlers, bus.bySource[e.Source().GetID()]...)
    }
    handlers = append(handlers, bus.global...)
    
    // Sort by priority and execute
    sort.Sort(byPriority(handlers))
    for _, h := range handlers {
        h.handler(ctx, e)
    }
    
    return nil
}
```

## This Solves Everything!

1. **No wasted function calls** - Bus pre-filters
2. **Cleaner feature code** - No if-checks for relevance
3. **Better performance** - O(1) handler lookup
4. **Clear intent** - Subscription declares what it cares about

The event bus becomes smart about routing events only to interested parties!