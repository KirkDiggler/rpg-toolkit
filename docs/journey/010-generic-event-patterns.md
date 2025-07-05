# Journey 006: Generic Event Patterns and Restoration Triggers

## The Problem

While implementing resource management (#30), we added methods like `ProcessShortRest()` and `ProcessLongRest()` directly to the Pool interface. This immediately felt wrong - we were baking D&D-specific rest mechanics into what should be generic infrastructure.

The question arose: "Should pools know about short rests?" The answer was clearly no, but then how should resources restore themselves?

## Initial Solutions Considered

### 1. Event-Driven with Hardcoded Checks
```go
pool.SubscribeToEvents(bus, []string{"rest.short", "rest.long"})
// Pool still knows about specific rest types
```
This still couples the pool to specific game mechanics.

### 2. Generic Triggers 
```go
pool.ProcessRestoration(trigger string, bus events.EventBus)
// Pool just forwards triggers to resources
```
Clean separation - pools don't know what triggers mean.

### 3. Strategy Pattern
```go
pool.ApplyRestoration(trigger string, strategy RestorationStrategy, bus)
// External strategies decide restoration
```
Powerful but adds complexity that rpg-toolkit doesn't need yet.

## The Breakthrough: Triggers as Generic Strings

The key insight: the Pool shouldn't know or care what triggers mean. It's just a dispatcher:

```go
func (p *SimplePool) ProcessRestoration(trigger string, bus events.EventBus) {
    for _, resource := range p.resources {
        if amount := resource.RestoreOnTrigger(trigger); amount > 0 {
            p.Restore(resource.Key(), amount, trigger, bus)
        }
    }
}
```

Resources are configured with trigger mappings:
```go
resource := NewSimpleResource(SimpleResourceConfig{
    RestoreTriggers: map[string]int{
        "my.game.short_rest": -1,  // full restore
        "my.game.dawn": 1,
        "my.game.milestone": 5,
    },
})
```

The game layer decides when to fire triggers:
```go
// Game-specific logic
eventBus.Subscribe("time.day_start", func(e events.Event) {
    pool.ProcessRestoration("divine.powers.refresh", bus)
})
```

## Broader Pattern: Cancellable Events

This discussion revealed a need for interruptible game flows:

```go
type CancellableEvent interface {
    events.Event
    Cancel()
    IsCancelled() bool
}

// Example: Spell saves
eventBus.Subscribe("spell.before_hit", func(e events.Event) {
    if SaveSucceeded() {
        e.(CancellableEvent).Cancel()
    }
})
```

This enables:
- Saving throws that prevent effects
- Resource checks that block actions
- Counterspells and interruptions
- Complex action resolution chains

## The Philosophy Reinforced

This journey reinforces rpg-toolkit's core philosophy:

1. **Infrastructure, not implementation**: We provide generic tools (ProcessRestoration), games provide meaning ("short_rest")
2. **Event-driven flexibility**: Games can wire up any restoration pattern - dawn, milestones, abilities, item use
3. **No assumptions**: We don't assume rests exist, or that resources restore at all
4. **Composition of simple parts**: Triggers + events + resources = any restoration system

## Concrete Benefits

1. **Unlimited restoration patterns**: Not just short/long rests
   - Spell that restores ally resources
   - Dawn/dusk restoration for divine powers
   - Milestone or story-based restoration
   - Combat end restoration
   - Any game-specific pattern

2. **Clean module boundaries**: Resources don't know about game rules, pools don't know about triggers
3. **Testable in isolation**: Each part can be tested without game context
4. **Future-proof**: New restoration patterns don't require toolkit changes

## Lessons Learned

- **Question specific methods**: ProcessShortRest() was a red flag
- **Strings are powerful abstractions**: Trigger names let games define meaning
- **Events enable everything**: Our event bus continues to be the key enabler
- **Keep pushing generic**: There's usually a more generic solution
- **Document the journey**: These discussions shape the architecture

## Next Steps

1. Implement generic restoration triggers (Issue #39)
2. Consider cancellable events as a broader pattern
3. Update resource examples to show various trigger patterns
4. Keep this pattern in mind for future modules

## The Bigger Picture

This pattern of "toolkit provides mechanism, game provides meaning" keeps appearing:
- Conditions apply effects, games define what "poisoned" means
- Proficiencies modify rolls, games define "Acrobatics"  
- Resources track consumption, games define "spell slots"
- Pools process triggers, games define "short rest"

This is the heart of rpg-toolkit: maximum flexibility through minimal assumptions.