---
name: events module
description: Type-safe pub/sub — EventBus, BusEffect, TypedTopic, ChainedTopic, StagedChain — and the chain pattern that drives combat resolution
updated: 2026-05-04
confidence: high — verified by reading all events/*.go and tracing the attack-chain worked example through rulebooks/dnd5e/combat/attack.go
---

# events module

**Path:** `events/`
**Module:** `github.com/KirkDiggler/rpg-toolkit/events`
**Grade:** B+

The pub/sub infrastructure that connects all game mechanics. Typed topics give
compile-time safety; chained topics implement the modifier pipeline that drives
combat resolution.

## What rpg-api consumes

Per audit Section 1, rpg-api imports the top-level `events` package from 4
files. The hot path is intentionally narrow:

| Symbol | Where rpg-api uses it | Histogram |
|---|---|---|
| `events.NewEventBus` | `internal/orchestrators/encounter/monster_turns.go` and other resolution paths | 10 |
| `events.EventBus` | helper return types (e.g. `internal/orchestrators/encounter/orchestrator.go`) | 1 |

rpg-api creates a fresh bus per attack/round of resolution (not one global bus)
and passes it into toolkit calls. **It does not subscribe handlers to the bus
itself** — that's a toolkit responsibility. Conditions, features, and combat
ability listeners are subscribed inside the toolkit (see "ConditionBehavior" in
`core.md`).

## Module surface

Exported symbols in `events/*.go` (verified by `grep -nE '^func [A-Z]|^type [A-Z]|^var [A-Z]'`):

| File | Exported |
|---|---|
| `bus.go` | `EventBus` interface, `NewEventBus()` |
| `bus_effect.go` | `BusEffect` interface (Apply / Remove / IsApplied) |
| `topic.go` | `Topic` (a `string` type — the routing key) |
| `topic_def.go` | `TypedTopicDef[T]`, `ChainedTopicDef[T]`, `DefineTypedTopic[T]`, `DefineChainedTopic[T]` |
| `typed_topic.go` | `TypedTopic[T]` interface |
| `chained_topic.go` | `ChainedTopic[T]` interface |
| `chain.go` | `StagedChain[T]`, `NewStagedChain[T]`, `ErrDuplicateID`, `ErrIDNotFound` |
| `errors.go` | `ErrDuplicateRef` |

`StagedChain[T]` implements `core/chain.Chain[T]` — the generic chain interface
lives in `core/chain/types.go`.

## EventBus

```go
type EventBus interface {
    Subscribe(ctx context.Context, topic Topic, handler any) (string, error)
    Unsubscribe(ctx context.Context, id string) error
    Publish(ctx context.Context, topic Topic, event any) error
}

func NewEventBus() EventBus { /* ... */ }
```

A plain pub/sub bus — `Subscribe` returns a subscription ID for cleanup. The
underlying implementation (`simpleEventBus`) is in-memory and goroutine-safe.

## TypedTopic — typed notification

```go
type TypedTopic[T any] interface {
    Subscribe(ctx context.Context, handler func(context.Context, T) error) (string, error)
    Unsubscribe(ctx context.Context, id string) error
    Publish(ctx context.Context, event T) error
}
```

A typed topic is a thin wrapper around `EventBus` for events of a known type
`T`. The `.On(bus)` pattern connects a topic-definition (compile-time singleton)
to a specific bus (runtime instance):

```go
turnEnds := dnd5eEvents.TurnEndTopic.On(bus)
turnEnds.Subscribe(ctx, handleTurnEnd)
turnEnds.Publish(ctx, dnd5eEvents.TurnEndEvent{ /* ... */ })
```

rpg-api uses this exact pattern at
`/home/kirk/personal/rpg-api/internal/orchestrators/encounter/orchestrator.go`
to publish `TurnEndEvent` after each turn (the only place rpg-api itself drives
a typed topic).

## ChainedTopic + StagedChain — the modifier pipeline

This is the load-bearing combat architecture. Per audit Section 3 Claim 4, the
chain pattern is the toolkit's answer to "Bless adds 1d4 to attacks, Rage adds
+2 to damage, Defense adds +1 to AC, all without those features knowing about
each other."

Three pieces cooperate:

1. **`core/chain.Chain[T]`** — the generic chain interface. Defined in
   `core/chain/types.go`: `Add(stage, id, modifier)`, `Remove(id)`,
   `Execute(ctx, T)`. Stage-ordered, ID-keyed.

2. **`events.StagedChain[T]`** — the concrete implementation. Built with
   `events.NewStagedChain(stages)`. Source: `events/chain.go`.

3. **`events.ChainedTopic[T]`** — the publish surface. Each subscriber receives
   the event and the running chain, may add modifiers, returns the chain.
   Source: `events/chained_topic.go`.

```go
type ChainedTopic[T any] interface {
    SubscribeWithChain(ctx context.Context,
        handler func(context.Context, T, chain.Chain[T]) (chain.Chain[T], error)) (string, error)
    PublishWithChain(ctx context.Context, event T, chain chain.Chain[T]) (chain.Chain[T], error)
    Unsubscribe(ctx context.Context, id string) error
}
```

### Worked example: the attack chain

The worked example lives in `rulebooks/dnd5e/combat/attack.go`. Stages are
defined in `rulebooks/dnd5e/combat/stages.go`:

```go
const (
    StageBase       chain.Stage = "base"        // dice rolls, proficiency, ability mod
    StageFeatures   chain.Stage = "features"    // rage damage, sneak attack
    StageConditions chain.Stage = "conditions"  // bless, bane, prone
    StageEquipment  chain.Stage = "equipment"   // magic weapon bonuses
    StageFinal      chain.Stage = "final"       // resistance/vulnerability, caps
)

var ModifierStages = []chain.Stage{
    StageBase, StageFeatures, StageConditions, StageEquipment, StageFinal,
}
```

`combat.ResolveAttack` (in `rulebooks/dnd5e/combat/attack.go`, search for the
`func ResolveAttack` symbol) builds the chain and publishes:

```go
attackChain := events.NewStagedChain[dnd5eEvents.AttackChainEvent](ModifierStages)
attacks := dnd5eEvents.AttackChain.On(input.EventBus)

// Publish through chain to collect modifiers
modifiedAttackChain, err := attacks.PublishWithChain(ctx, attackEvent, attackChain)
// Execute chain to get final attack event with all modifiers
finalAttackEvent, err := modifiedAttackChain.Execute(ctx, attackEvent)
```

Subscribers along the chain include condition handlers (e.g.
`RagingCondition.onDamageReceived` in `rulebooks/dnd5e/conditions/raging.go`)
that were registered when the condition was Apply'd to the bus. After the
chain returns, `ResolveAttack` resolves hit/damage and publishes a
`DamageReceivedEvent` so condition handlers (rage resistance, etc.) modify
incoming damage before it lands.

## BusEffect — the lifecycle interface

```go
type BusEffect interface {
    Apply(ctx context.Context, bus EventBus) error
    Remove(ctx context.Context, bus EventBus) error
    IsApplied() bool
}
```

`BusEffect` is the toolkit-internal pattern for any mechanic that needs to
subscribe and unsubscribe as it becomes active/inactive. `dnd5eEvents.ConditionBehavior`
(in `rulebooks/dnd5e/events/events.go`) is a near-clone with one extra method
(`ToJSON`) for the per-condition serialization pattern. Conditions in the
dnd5e rulebook implement `ConditionBehavior`, not `BusEffect` directly — but
the lifecycle is identical.

`BusEffect` is exported, so any consumer that imports `events` could reference
it — but **rpg-api does not import it**. It's the contract toolkit-internal
mechanics use to manage their bus subscriptions, and per the audit rpg-api
relies on `EventBus` and `NewEventBus` only.

## Rewrite history (issue #617)

The events module was rewritten from a typed-event API (`Event`,
`HandlerFunc`, `event.Context().GetString()`, `event.Context().AddModifier()`)
to the current typed-topic API (`TypedTopic[T]`, `ChainedTopic[T]`,
`BusEffect`, `StagedChain`). Several modules under `mechanics/*`
(`mechanics/conditions`, `mechanics/effects`, `mechanics/features`,
`mechanics/spells`) still carry source written against the old shape — they
reference `events.Event` and `events.HandlerFunc` symbols that no longer
exist on either the local or any published events module. Those modules
carry `replace` directives in their `go.mod` pointing `events => ../../events`,
but since the local `events` only exposes the typed-topic API, the directives
don't actually let those modules build against the old symbols — running
`go mod tidy` or `go build ./...` inside them fails. The directives are a
holdover from before the events rewrite landed.

Closing #617 means rewriting those mechanics modules against the typed-topic
API — a real refactor, not a version bump. The 4-class playtest doesn't
exercise them directly (rpg-api consumes their refactored cousins through
`rulebooks/dnd5e/*`), so the migration is deferred. See `mechanics.md` and
the components-doc audit
(`docs/journey/049-rpg-api-toolkit-usage-audit.md`) for the consumer view.

## Known gaps

- **No ADR for the EventBus vs. BusEffect split.** New contributors will default to whichever they find first. The right heuristic: use `EventBus` for stateless observers; use `BusEffect` (or `ConditionBehavior` inside dnd5e) for stateful game mechanics that have a lifecycle (applied/removed).
- Example tests (`example_journey_test.go`, `example_magic_test.go`) are not in suite pattern — acceptable for examples but inconsistent with the rest of the repo.
- Version spread across the toolkit: the `game` module pins `events v0.1.1` while newer modules use `v0.6.2`. Until #617 closes, mixing modules with different events versions can produce subtle subscription-shape mismatches.

## Verification

```sh
# Module surface
grep -nE "^func [A-Z]|^type [A-Z]|^var [A-Z]" /home/kirk/personal/rpg-toolkit/events/*.go | grep -v _test

# Worked example
grep -n 'func ResolveAttack\|PublishWithChain\|NewStagedChain' /home/kirk/personal/rpg-toolkit/rulebooks/dnd5e/combat/attack.go

# Stage definitions
cat /home/kirk/personal/rpg-toolkit/rulebooks/dnd5e/combat/stages.go

# rpg-api's events surface
grep -rn '"github.com/KirkDiggler/rpg-toolkit/events"' /home/kirk/personal/rpg-api/internal/ /home/kirk/personal/rpg-api/cmd/ --include="*.go" | wc -l   # expect 4
```
