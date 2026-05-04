# ADR-0006: Generic Restoration Triggers

## Status
Proposed

## Context
During implementation of the resource management system (#30), we added `ProcessShortRest()` and `ProcessLongRest()` methods directly to the Pool interface. This couples D&D-specific rest mechanics to what should be generic infrastructure.

Resources need ways to restore themselves based on game events, but the toolkit shouldn't assume what those events are.

## Decision
Replace specific restoration methods with a generic trigger system:

```go
// Instead of:
pool.ProcessShortRest(bus)
pool.ProcessLongRest(bus) 

// Use:
pool.ProcessRestoration(trigger string, bus events.EventBus)
```

Resources will be configured with trigger-to-restoration mappings:
```go
resource := NewSimpleResource(SimpleResourceConfig{
    RestoreTriggers: map[string]int{
        "my.game.short_rest": -1,  // full restore
        "my.game.dawn": 1,
        "custom.ability.used": 3,
    },
})
```

## Consequences

### Positive
- Pools remain generic infrastructure with no game-specific knowledge
- Games can implement any restoration pattern (dawn, milestone, ability-triggered, etc.)
- Resources are configured declaratively with trigger mappings
- Supports game-specific trigger names without toolkit changes
- Maintains clean module boundaries

### Negative  
- Slightly more configuration needed when creating resources
- Games must standardize their trigger names
- One more concept for developers to understand

### Neutral
- Existing code using ProcessShortRest/ProcessLongRest will need migration
- Opens the door for more event-driven patterns (cancellable events, event chains)

## Implementation Notes
1. Add `ProcessRestoration(trigger string, bus EventBus)` to Pool interface
2. Add `RestoreOnTrigger(trigger string) int` to Resource interface  
3. Update SimpleResource to support trigger configuration
4. Deprecate but don't immediately remove ProcessShortRest/ProcessLongRest
5. Update examples to show various trigger patterns

## Related
- Journey 006: Generic Event Patterns and Restoration Triggers
- Issue #39: Refactor generic restoration triggers for resource pools