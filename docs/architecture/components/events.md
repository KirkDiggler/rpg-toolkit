---
name: events module
description: Type-safe pub/sub event bus — EventBus, BusEffect, TypedTopic, ChainedTopic
updated: 2026-05-02
confidence: high — verified by reading bus.go, bus_effect.go, typed_topic.go, chained_topic.go, chain.go
---

# events module

**Path:** `events/`
**Module:** `github.com/KirkDiggler/rpg-toolkit/events`
**Grade:** B+

The pub/sub infrastructure that connects all game mechanics. Typed topics give compile-time safety; chained topics implement the modifier pipeline (the "chain pattern").

## Files

| File | Purpose |
|---|---|
| `bus.go` | `EventBus` interface + `NewEventBus()` |
| `bus_effect.go` | `BusEffect` interface — subscribe/unsubscribe lifecycle |
| `topic.go` | `Topic` interface |
| `topic_def.go` | `TopicDef` — named topic definitions |
| `typed_topic.go` | `TypedTopic[T]` — generic typed topic |
| `chained_topic.go` | `ChainedTopic` — modifier chain topic |
| `chain.go` | `Chain`, `ChainResult` types |
| `errors.go` | Event-specific errors |
| `CHAIN_PATTERN.md` | In-tree documentation of the chain pattern |

## The dual-bus pattern

There are two distinct concepts in this module:

**`EventBus`** (`bus.go`) — plain notification bus. Any subscriber on a topic gets called when an event is published. Used for "something happened" notifications (entity moved, condition applied, turn ended).

**`BusEffect`** (`bus_effect.go`) — a lifecycle interface for game mechanics that need to subscribe and unsubscribe as they become active/inactive:
```go
type BusEffect interface {
    Apply(bus EventBus) error    // subscribe handlers, mark active
    Remove(bus EventBus) error   // unsubscribe handlers, mark inactive
    IsApplied() bool
}
```

A `RagingCondition` implements `BusEffect`. When Rage activates, `Apply()` subscribes a damage modifier handler on the damage chain topic. When Rage ends, `Remove()` unsubscribes it. The character's condition slice stores `BusEffect` implementations.

**The dual-bus split is not documented in any ADR.** ADR-0024 covers typed topics but not when to use `EventBus` directly vs. routing through `BusEffect`. This is a real contributor gap.

## TypedTopic

```go
type TypedTopic[T any] interface {
    Topic
    On(bus EventBus) BoundTopic[T]  // bind to a specific bus
}

// Usage
attacks := combat.AttackTopic.On(bus)
attacks.Subscribe(ctx, handleAttack)
attacks.Publish(ctx, attackEvent)
```

`On(bus)` returns a `BoundTopic[T]` — a topic bound to a specific event bus instance. This makes the bus connection explicit and allows the same topic definition to work with different buses (e.g., per-encounter bus vs. global bus).

## ChainedTopic

The modifier pipeline. Each handler receives the running chain, adds its modifier, and returns the updated chain. The final handler collects the accumulated `Breakdown`.

```go
// A D&D 5e attack roll chain might look like:
// base roll → proficiency bonus → ability modifier → condition modifiers → final total
attackRollChain := ChainedTopic[AttackRollChainEvent]
```

All the "chain" references in `overview.md` refer to this mechanism. The chain pattern is what makes "Bless adds 1d4" work without the bless implementation knowing about attack resolution.

## Known gaps

- **No ADR for the EventBus vs. BusEffect split.** New contributors will default to whichever they find first. The right heuristic: use `EventBus` for stateless observers; use `BusEffect` for stateful game mechanics that have a lifecycle (applied/removed).
- Example tests (`example_journey_test.go`, `example_magic_test.go`) are not in suite pattern — acceptable for examples but inconsistent with the rest of the repo.
- `events v0.1.1` is pinned by the `game` module while newer modules use `v0.6.2`. This version spread means `game.Context` passes older `EventBus` types than what `tools/spatial` expects.
