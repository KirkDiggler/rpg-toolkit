# Journey 014: Event Bus Evolution

## The Discovery

While adding context support to the events module and preparing for pipeline integration, we discovered several architectural issues and opportunities in our event bus implementation.

## Initial State

The event bus was using:
- Pointer comparison for ref matching (`entry.ref != event.EventRef()`)
- Reflection for handler signatures with backward compatibility
- Read lock held during handler execution
- Mixed return types (error or DeferredAction)
- Legacy SubscribeFunc method creating mismatched refs

## Problems Identified

### 1. Ref Matching Was Fundamentally Broken

The bus compared refs by pointer (`entry.ref != event.EventRef()`), meaning:
- Events had to return the exact same ref instance to match handlers
- SubscribeFunc created new refs that would never match
- Same logical ref with different instances wouldn't match

### 2. Lock Contention

The read lock was held during all handler execution:
- Blocked new subscriptions during event processing
- Could cause deadlocks if handlers tried to modify the bus
- Limited concurrency

### 3. Unnecessary Complexity

- Reflection overhead for every handler call
- Backward compatibility code for different handler signatures
- Dual context systems (EventContext and context.Context)

## Solutions Implemented

### 1. Fixed Ref Matching

Changed from pointer comparison to value comparison:
```go
// Before: if entry.ref != event.EventRef()
// After: Group by ref.String(), match by value
refStr := event.EventRef().String()
if entries, ok := b.handlers[refStr]; ok {
    // Process matching handlers
}
```

### 2. Lock-Free Publishing

Copy handlers before execution:
```go
// Phase 1: Copy handlers with read lock
var handlersToCall []handlerEntry
b.mu.RLock()
// ... copy matching handlers ...
b.mu.RUnlock()

// Phase 2: Call handlers without lock
for _, entry := range handlersToCall {
    // Call handler
}
```

Benefits:
- No deadlocks during handler execution
- Handlers can subscribe/unsubscribe during events
- Better concurrency

### 3. Simplified Architecture

- Removed all backward compatibility
- Standardized on `func(context.Context, T) error` signature
- Removed SubscribeFunc method
- All handlers now require context

## Architectural Insights

### Event Bus as Transport

The event bus should focus on transport responsibilities:
- Routing events to handlers (done)
- Managing subscriptions (done)
- Ensuring delivery (done)
- Preventing cascading depth issues (done)

### Pipeline Integration Point

Events can trigger pipelines, but the bus remains the transport:
- Events are "what happened"
- Pipelines are "how to process it"
- Bus delivers events to pipeline triggers
- Clean separation of concerns

### Future Opportunities

1. **Type-Safe Generic Bus**: Create `TypedBus[T Event]` for compile-time safety
2. **Context Unification**: Merge EventContext with context.Context
3. **Pre-computed Routing**: Cache handler lists per ref for performance
4. **Priority/Phases**: Add execution order control (when needed)

## Testing Insights

Created comprehensive concurrent tests proving:
- Handlers can subscribe during event processing
- Handlers can unsubscribe during event processing
- Multiple goroutines can publish simultaneously
- No deadlocks or race conditions

## Key Decisions

1. **Value-based ref matching**: More intuitive and correct
2. **Lock-free handler execution**: Better concurrency, no deadlocks
3. **Remove legacy code**: Cleaner, simpler implementation
4. **Context everywhere**: Consistent patterns

## Lessons Learned

1. **Question inherited decisions**: The pointer comparison was "baggage we brought along"
2. **Simplify when possible**: Removing backward compatibility made everything cleaner
3. **Test concurrent behavior**: Lock-free doesn't mean correct without tests
4. **Separate concerns**: Event bus is transport, not business logic

## Impact on Pipeline Architecture

This evolution prepares us for pipeline integration:
- Clean event delivery mechanism
- No lock contention during pipeline execution
- Context flows naturally through the system
- Events can trigger pipelines without coupling

## Next Steps

1. Create type-safe generic event bus (when optimization needed)
2. Consider context unification (EventContext + context.Context)
3. Implement pipeline triggers on top of clean event transport
4. Document patterns for event-driven pipelines

## Conclusion

By questioning our assumptions and removing accumulated complexity, we transformed the event bus from a fragile, lock-heavy system to a clean, concurrent transport layer ready for pipeline integration. The key insight: **The subscription defines what it wants to receive, not the event defining where it goes.**