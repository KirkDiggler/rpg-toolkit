---
name: encounter module
description: Orchestrator-facing SDK for running an encounter end-to-end — sealed event taxonomy, process-scoped Broker, transient Encounter aggregate, discrete-phase combat orchestration
updated: 2026-05-14
confidence: high — Wave 2.11d shipped the discrete-phase combat surface verified by shipped code + tests + ADR-0027 cross-reference
---

# encounter module

**Path:** `encounter/`
**Module:** `github.com/KirkDiggler/rpg-toolkit/encounter`
**Grade:** B+ (Wave 2.11d added discrete-phase combat orchestration; grade was B at first slice)

The encounter SDK is the orchestrator-facing facade for running an encounter
(combat, free-roam, social) end-to-end. Game servers `Load` an encounter from
persisted state, mutate via verb methods (`Move`, `OpenDoor`, ...), serialize
back via `ToData`, and save. Player-facing events flow through a process-scoped
`Broker` that publishes per-player projected events through a pluggable
`Transport`.

## Internal layout

One Go module with three subpackages forming a linear DAG (`core ← events ← perception ← encounter`):

- `encounter/core` — identity primitives (`EncounterID`, `PlayerID`, `EntityID`) and spatial primitives (`Hex`, `HexSet`). Exists to break the encounter↔events package import cycle. `HexSet` has custom `MarshalJSON`/`UnmarshalJSON` (struct map keys can't serialize via the default codec). `Hex`/`HexSet` may move to `tools/spatial` in a future slice once the encounter SDK is ready to depend on the spatial module directly.
- `encounter/events` — sealed `EncounterEvent` interface, three concrete events (`MoveEvent`, `HexRevealedEvent`, `DoorOpenedEvent`), and `AudienceSet` (event-routing concept; lives with events).
- `encounter/events` — sealed `EncounterEvent` interface (AWS v2 SDK marker pattern: unexported `isEncounterEvent()` makes the interface externally unsatisfiable). Concrete events implemented in slice 1: `MoveEvent`, `HexRevealedEvent`, `DoorOpenedEvent`. Each has its own `MarshalJSON`/`UnmarshalJSON` so unexported `encID`/`seq` fields round-trip without leaking construction-only state.
- `encounter/perception` — pure projection functions (`ProjectMove`, `ProjectDoorOpen`) and `View` value type. Stub LoS today (Manhattan radius); real LoS is a future slice.
- `encounter` (top-level) — `Encounter` aggregate, `Broker`, `Transport`, `InMemoryTransport`, JSON codec. The `Broker` is process-scoped — one per game-server process — and uses `sync.WaitGroup` to ensure listener goroutines exit before subscription channels close on shutdown (no double-close races).

## Key types

- `Encounter` — transient. Constructed per-call from `Data`. Verbs (`Move`, `OpenDoor`) compute per-player projections, mutate state, publish events.
- `Data` — persisted shape. Carries Players (with `View`), Doors, monotonic Sequence counter.
- `Broker` — process-scoped, holds in-process subscription registry (keyed by `(encID, playerID)`), routes via Transport.
- `EncounterEvent` — sealed sum interface. Concrete events implement `isEncounterEvent()`, `EncounterID()`, `Sequence()`, `Audience()`.
- `Transport` — pluggable bytes-level pub/sub. Channel keys opaque (`enc:<id>`); payloads opaque bytes; encoding is the Broker's concern.

## Cause vs effect events

Action events (cause) describe what happened in the world — `MoveEvent`, `DoorOpenedEvent`. Effect events describe perception changes — `HexRevealedEvent`. **The two are decoupled**: any cause that changes vision (Move, OpenDoor, future LightChanged, future ConditionRemoved) emits the same `HexRevealedEvent` shape alongside its action event. New cause types don't touch existing event types. Symmetric `HexHiddenEvent` is reserved for vision-loss cases (walking out of LoS, lights out, gaining Blinded) — not in slice 1.

## Discrete-phase combat orchestration (Wave 2.11d)

Combat resolution that may involve player reactions is modeled as **two RPC-spanning phases** rather than a single in-process call. The SDK does not pause the chain itself; each phase runs end-to-end. The "pause" lives between SDK verb invocations — the host calls phase 1, gets back any pending reaction prompts, awaits the player's `SubmitCheck`, then calls phase 2.

The SDK exposes the two phases as `TakeActionPhased` and `CompleteTakeAction` verbs on `Encounter`. Both delegate to an optional `PhasedCombatResolver` interface that extends the existing `CombatResolver` interface — hosts that supply only a base `CombatResolver` get the legacy single-call path. The legacy `TakeAction` verb now wraps `TakeActionPhased`, so existing call sites get the new orchestration for free.

```
host ──► Encounter.TakeActionPhased ──► resolver.ResolveAttackHit (chain runs)
                                   │
                                   ▼
                                   pending ReactionTrigger events drained
                                   ▼
                                   PendingReactionPrompts stored on Data
                                   ▼
host◄── TakeActionOutcome (pending prompts + InputRequired events)
   │
   │  ... player submits SubmitCheck{take_reaction: bool} ...
   ▼
host ──► Encounter.CompleteTakeAction ──► resolver.ApplyAttackOutcome (chain runs again with reaction modifiers baked in)
```

**Buffered subscriber drain.** Phase 1 installs an inline buffered subscriber on the encounter bus that collects every `ReactionTriggerEvent` published during the chain. The buffer is protected by a `sync.Mutex` — today's bus dispatches handlers in the publisher's goroutine, but the mutex makes the pattern safe against a future fan-out bus implementation and matches the helper pattern used in `opportunity_attack_test.go` / `shield_spell_test.go`. After the chain returns, the orchestrator partitions buffered triggers by reactor: player reactors get a `PendingReactionPrompt` written to Data; NPC reactors are resolved inline by walking the captured triggers and applying any auto-resolve outcomes against the live attack context.

**Phase 2 inline-vs-resumed.** If phase 1 surfaced no player reactors with ready reactions, `TakeActionPhased` calls `CompleteTakeAction` synchronously before returning — the host sees a single round-trip and the player path is untouched. If a player reactor was found, the SDK returns the pending prompts and the orchestrator waits for `SubmitCheck` before invoking `CompleteTakeAction`. The split lives entirely at the SDK verb boundary; the chain itself never pauses.

### PendingReactionPrompts persistence

Phase 1 outcomes that need to survive an RPC gap are written to `Encounter.Data.PendingReactionPrompts` — a map keyed by reactor `PlayerID`. Each `PendingReactionPrompt` carries:

- `PromptID` — host-side correlation token for matching the player's `SubmitCheck`.
- `ReactionRef` — which reaction is being offered (Shield, OpportunityAttack, etc.).
- `TriggerEvent` — the `ReactionTriggerEvent` payload, serialized.
- `AttackContextJSON` — opaque bytes the rulebook adapter marshaled from `combat.AttackContext`.
- `Deadline` / `MaxWaitMillis` — host-supplied turn-clock policy.

When the orchestrator resumes via `CompleteTakeAction`, the SDK reads the prompt back out of Data, unmarshals `AttackContextJSON` via the host's resolver adapter, and feeds the rehydrated `AttackContext` into phase 2.

**HOST CONTRACT (tracked in [#657](https://github.com/KirkDiggler/rpg-toolkit/issues/657)):** the NPC-pause path writes `AttackContextJSON: nil` and relies on the host to populate it before snapshotting. The host must detect `IsNPCPausedForReaction(err)` from `TakeActionPhased`, walk `Encounter.Data.PendingReactionPrompts` for entries with empty `AttackContextJSON`, fetch the live `*PhasedAttackContext` via `Encounter.PendingPhasedAttackContext(playerID)`, marshal it through the rulebook adapter, write the bytes back, and only then call `ToData()`. rpg-api's `serializePendingPhasedAttacks` is the reference implementation. The proper long-term fix is a resolver-supplied serializer callback so the SDK populates the bytes itself — issue #657 tracks it.

### InputRequiredDeliveredEvent

`encounter/events.InputRequiredDeliveredEvent` is the bus signal the wire-side translator listens for to know a reaction prompt is ready for the reactor. **Metadata-only** — the event carries only `PromptID` + the reactor's `PlayerID` and an audience set of one (the reactor alone). The translator reads `Encounter.Data.PendingReactionPrompts` to build the proto payload; the event itself never carries the prompt body. This keeps the SDK event payload small enough to fit in any transport message-size budget while letting the host's projection layer compose the full proto from the canonical Data shape.

### NPC pause sentinel

When `NPCAct` invokes the phased path and a player reactor has a triggered prompt, the NPC's turn pauses by returning the unexported sentinel `errNPCPausedForReaction` from `applyCapturedAttacks`. Hosts detect it via:

```go
err := encounter.NPCAct(ctx, npcID)
if encounter.IsNPCPausedForReaction(err) {
    // serialize pending reaction prompts, snapshot, await SubmitCheck
} else if err != nil {
    // real error
}
```

`IsNPCPausedForReaction` uses `errors.Is` so the helper survives any `%w`-wrapping callers add. The sentinel is unexported deliberately — the helper is the only legitimate detection path.

## Implementation notes worth keeping

Three lessons surfaced while building slice 1 that are likely to bite future toolkit work:

### Go's `encoding/json` cannot serialize struct-keyed maps

`HexSet` is `map[Hex]struct{}`. The default codec emits an empty `{}` for struct keys — silently. A round-trip through JSON loses every entry, so any persisted state that crossed `ToData`/`LoadFromData` would have empty fog-of-war. `HexSet` ships custom `MarshalJSON`/`UnmarshalJSON` that serialize as a sorted `[]Hex`. Any future struct-keyed sets in toolkit will hit the same trap.

### Fanout broker shutdown / close race

The original Broker design released the registry mutex before sending events to per-subscriber channels (snapshot subscribers under lock, send outside). That seems sensible — slow consumers don't stall the listener — but it races with `Subscription.Close()` closing those channels. Send-on-closed-channel panics. Fix: hold the lock through the fanout sends. Sends are non-blocking (`select+default`) so the held duration is bounded. Same shape applies to `InMemoryTransport.Publish` over its subscriber list. Anywhere a fan-out goroutine sends to channels owned by external close paths, the lock must cover the send.

### Cycle pressure is design pressure

The first cut of this module had a `types/` subpackage holding everything that needed to live below both `encounter` and `encounter/events`. That broke the import cycle but obscured the design — `types/` was a generic bucket clumping unrelated primitives (IDs, hex coords, audience routing). The reshape gave each kind its right home: `core/` for identity + spatial primitives, `AudienceSet` moved into `events/` where it belongs as a routing concept. When you're tempted to create a `types/` or `common/` subpackage to break a cycle, treat it as a smell first — the cycle may be telling you to organize, not just to deduplicate location.

## Spec / plan references

- Spec: `rpg-project/ideas/encounter/v1alpha2/sdk-direction.md`
- Slice plan: `rpg-project/ideas/encounter/v1alpha2/plans/02-walking-skeleton.md`

## Out of scope (slice 1) — partially shipped by Wave 2.11d

The original slice-1 cut-list deferred combat verbs and reaction
handling. Wave 2.11d shipped the combat slice of that list:

- **Shipped (Wave 2.11d):** `TakeActionPhased` + `CompleteTakeAction`
  combat verbs, `PhasedCombatResolver` extension interface,
  `PendingReactionPrompts` persistence, `InputRequiredDeliveredEvent`
  reaction-prompt-delivery event, NPC pause sentinel.

- **Still future:** `ActivateFeature`, `UseAction`, `Interact`,
  `SubmitCheck` (lives on rpg-api today; the SDK only sees the resumed
  attack flow via `CompleteTakeAction`), `EndTurn` (lives on rpg-api),
  action economy beyond what `combatabilities` ships, conditions beyond
  the dnd5e rulebook's set, senses, real LoS (still Manhattan stub),
  Redis transport, gRPC handler. Entity-visibility accumulation is
  reserved in the type shapes (`HexRevealedSlice.Entities`,
  `View.KnownEntities`) but not emitted yet — future slice.

Catch-up policy: snapshot-only on reconnect (load `Snapshot`, attach live stream). Event-replay catch-up is a future slice that adds an `EventLog` interface alongside `Transport`. Sequence numbers on events are already in place; the addition is non-breaking.
