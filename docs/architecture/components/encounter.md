---
name: encounter module
description: Orchestrator-facing SDK for running an encounter end-to-end — sealed event taxonomy, process-scoped Broker, transient Encounter aggregate
updated: 2026-05-06
confidence: medium — first slice (walking skeleton) just landed; expanded grades will follow as combat verbs and real LoS land
---

# encounter module

**Path:** `encounter/`
**Module:** `github.com/KirkDiggler/rpg-toolkit/encounter`
**Grade:** B (first slice; tests pass, narrow scope by design — grade rises as more verbs land)

The encounter SDK is the orchestrator-facing facade for running an encounter
(combat, free-roam, social) end-to-end. Game servers `Load` an encounter from
persisted state, mutate via verb methods (`Move`, `OpenDoor`, ...), serialize
back via `ToData`, and save. Player-facing events flow through a process-scoped
`Broker` that publishes per-player projected events through a pluggable
`Transport`.

## Internal layout

One Go module with four subpackages forming a linear DAG (`types ← events ← perception ← encounter`):

- `encounter/types` — primitive value types (`EncounterID`, `PlayerID`, `EntityID`, `Hex`, `HexSet`, `AudienceSet`). Exists to break the encounter↔events package import cycle. `HexSet` has custom `MarshalJSON`/`UnmarshalJSON` (struct map keys can't serialize via the default codec).
- `encounter/events` — sealed `EncounterEvent` interface (AWS v2 SDK marker pattern: unexported `isEncounterEvent()` makes the interface externally unsatisfiable). Concrete events implemented in slice 1: `MoveEvent`, `HexRevealedEvent`, `DoorOpenedEvent`. Each has its own `MarshalJSON`/`UnmarshalJSON` so unexported `encID`/`seq` fields round-trip without leaking construction-only state.
- `encounter/perception` — pure projection functions (`ProjectMove`, `ProjectDoorOpen`) and `PerceptionView` value type. Stub LoS today (Manhattan radius); real LoS is a future slice.
- `encounter` (top-level) — `Encounter` aggregate, `Broker`, `Transport`, `InMemoryTransport`, JSON codec. The `Broker` is process-scoped — one per game-server process — and uses `sync.WaitGroup` to ensure listener goroutines exit before subscription channels close on shutdown (no double-close races).

## Key types

- `Encounter` — transient. Constructed per-call from `EncounterData`. Verbs (`Move`, `OpenDoor`) compute per-player projections, mutate state, publish events.
- `EncounterData` — persisted shape. Carries Players (with `PerceptionView`), Doors, monotonic Sequence counter.
- `Broker` — process-scoped, holds in-process subscription registry (keyed by `(encID, playerID)`), routes via Transport.
- `EncounterEvent` — sealed sum interface. Concrete events implement `isEncounterEvent()`, `EncounterID()`, `Sequence()`, `Audience()`.
- `Transport` — pluggable bytes-level pub/sub. Channel keys opaque (`enc:<id>`); payloads opaque bytes; encoding is the Broker's concern.

## Cause vs effect events

Action events (cause) describe what happened in the world — `MoveEvent`, `DoorOpenedEvent`. Effect events describe perception changes — `HexRevealedEvent`. **The two are decoupled**: any cause that changes vision (Move, OpenDoor, future LightChanged, future ConditionRemoved) emits the same `HexRevealedEvent` shape alongside its action event. New cause types don't touch existing event types. Symmetric `HexHiddenEvent` is reserved for vision-loss cases (walking out of LoS, lights out, gaining Blinded) — not in slice 1.

## Spec / plan references

- Spec: `rpg-project/ideas/encounter/v1alpha2/sdk-direction.md`
- Slice plan: `rpg-project/ideas/encounter/v1alpha2/plans/02-walking-skeleton.md`

## Out of scope (slice 1)

Combat verbs (`Attack`, `ActivateFeature`, `UseAction`, `Interact`, `SubmitCheck`, `EndTurn`), action economy, conditions, senses, real LoS, Redis transport, gRPC handler, monsters, all event types beyond Move/HexRevealed/DoorOpened. Entity-visibility accumulation is reserved in the type shapes (`HexRevealedSlice.Entities`, `PerceptionView.KnownEntities`) but not emitted yet — future slice.

Catch-up policy: snapshot-only on reconnect (load `Snapshot`, attach live stream). Event-replay catch-up is a future slice that adds an `EventLog` interface alongside `Transport`. Sequence numbers on events are already in place; the addition is non-breaking.
