---
name: encounter module
description: Orchestrator-facing SDK for running an encounter end-to-end — sealed event taxonomy, process-scoped Broker, transient Encounter aggregate, combatant hydration cascade, discrete-phase combat orchestration, MovementResolver seam (both movement directions)
updated: 2026-05-30
confidence: high — #689 made LoadFromData own combatant hydration (the #684 double-subscribe cure); Wave 2.11d shipped discrete-phase combat; Wave 2.11e extended CompleteTakeAction to accept either PvE attack direction AND added MovementResolver for per-step movement in BOTH directions
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
- `Broker` — process-scoped, holds in-process subscription registry (keyed by `(encID, playerID)`), routes via Transport. The **single publish authority**: `Broker.Publish` stamps each event's game-event time from an injected `core.Clock` (default `SystemClock`; `NewBrokerWithClock` injects a `FixedClock` in tests) at the literal publish moment, preserving any correlation id the encounter set on it. See ADR-0031.
- `EncounterEvent` — sealed sum interface. Concrete events implement `isEncounterEvent()`, `EncounterID()`, `Sequence()`, `Audience()`, plus the spine metadata `OccurredAt()` (game-event time, Inv 5), `CorrelationID()` (causation grouping, Inv 8), and `Stamp(at, corr)`. The last three are single-sourced on the embedded `eventMeta` so adding a spine field touches one struct, not every event.
- `ActionResolvedEvent` — first-class "an action was taken" event (Inv 9): `ActorID`, `ActionRef` (string), optional `TargetID`, and `EconomyConsumed` (actions/bonus/reactions/movement + a `granted_consumed` map). Emitted for every player-facing action as the cause beat; attack-specific roll detail stays on the parallel `AttackResolvedEvent`. The effect events of one action share the `ActionResolvedEvent`'s correlation id so the toolkit-owned combat log is reassemblable.
- `Transport` — pluggable bytes-level pub/sub. Channel keys opaque (`enc:<id>`); payloads opaque bytes; encoding is the Broker's concern.

## Cause vs effect events

Action events (cause) describe what happened in the world — `MoveEvent`, `DoorOpenedEvent`, `ActionResolvedEvent`. Effect events describe perception/state changes — `HexRevealedEvent`, `DamageDealtEvent`, `ConditionAppliedEvent`. **The two are decoupled**: any cause that changes vision (Move, OpenDoor, future LightChanged, future ConditionRemoved) emits the same `HexRevealedEvent` shape alongside its action event. New cause types don't touch existing event types. Symmetric `HexHiddenEvent` is reserved for vision-loss cases (walking out of LoS, lights out, gaining Blinded) — not in slice 1.

**Causation (Inv 8):** the `ActionResolvedEvent` and every effect event it produces in one resolution carry the same `CorrelationID` (derived from the resolved-action event's `(encID, sequence)` identity — deterministic, rides the existing monotonic sequence, no extra dependency). A consumer reassembles "this damage came from that attack" from the correlation id, not from adjacent sequence numbers. See ADR-0031.

## Combatant hydration cascade (#689)

`Encounter.LoadFromData(ctx, ...)` owns combatant hydration: it cascades into
each combatant's own `LoadFromData` — players from `PlayerData.DataJSON` via
`character.LoadFromData`, monsters from `MonsterData.DataJSON` via
`monster.LoadFromData` + `monsteractions.LoadMonsterActions` +
`monstertraits.LoadMonsterConditions` — and applies default reaction conditions
(OA/Shield, driven by the `ReactionReadiness` map) in the same step. The runtime
entities are **held** on the `Encounter` as `combat.Combatant` (reconstructed,
not serialized, each load — like the bus).

This single cascade is the **only** place conditions `Apply` to the encounter
bus: one load, one subscribe. It cures the #684 "modifier ID already exists"
double-subscribe class that arose when the host re-loaded entities per attack and
per turn-boundary, each re-`Apply`'ing conditions to the same bus.

- **Resolvers receive the held entity.** `AttackInput.Attacker/Defender` and
  `MovementStepInput.Mover` are `combat.Combatant`; resolvers use them and MUST
  NOT re-load. Nil when a seat carried no rehydratable data → resolver falls back
  to its stat-snapshot stand-in.
- **`EndTurn(ctx, ...)` emits the turn-boundary** (`dnd5eEvents.TurnEndTopic`)
  directly on the bus for the ending actor, so held conditions reset per-turn
  state (`SneakAttack.UsedThisTurn`) in place with no re-load.
- **`ToData()` mirrors the cascade**: it re-serializes each held entity's
  `ToData()` back into the owning `PlayerData/MonsterData.DataJSON` (unconditional
  — `IsDirty()` tracks only HP, not condition state; see ADR-0030), so the next
  load sees current state. The SDK still never *stores* — the host saves the
  returned `Data`.
- **`NPCAct`** reuses the cascade-held monster; it only loads when there is no
  held instance (the `New`-without-`LoadFromData` path).

Boundary note: the SDK is dnd5e-coupled by precedent (`npc.go`/`activate_feature.go`
already called the rulebook loaders); #689 makes that coupling single-sourced
rather than scattered across the host. See ADR-0030 + journey 050.

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

### NPC-attacker resume direction (Wave 2.11e)

`CompleteTakeAction` accepts either PvE attack direction. The shipped Wave 2.11d shape rejected NPC attackers (the original implementation looked up `attackCtx.AttackerID` only against `data.Players`), which broke the resume path for the only direction Shield can fire in: monster attacks player → player Shield prompt → player chooses Take → resume calls `CompleteTakeAction` with `AttackerID = monsterID`.

The Wave 2.11e fix resolves direction polymorphically by checking the AttackerID against both the Players map and the Monsters map, then dispatching to the appropriate publish helper:

| Direction | AttackerID resolved against | Outcome publish path | Death event |
|---|---|---|---|
| player→monster | `data.Players` | `applyAndPublishOutcome(player, monster, outcome)` | `killEntity` (full kill chain — remove from initiative, check encounter-end) |
| monster→player | `data.Monsters` | `applyAndPublishNPCOutcome(monster, player, outcome)` | `publishPlayerDied` (Wave 2.10 partial — event only, no removal, no encounter-end) |
| player→player | n/a | rejected with `ErrUnsupportedAttackDirection` | n/a |
| monster→monster | n/a | rejected with `ErrUnsupportedAttackDirection` | n/a |

`applyAndPublishNPCOutcome` is extracted from the per-attack body of `applyCapturedAttacks` (encounter/npc.go) and re-used from both the inline NPC turn and the Shield-resume direction so the two paths emit identical event shapes. Before this extraction, the inline `applyCapturedAttacks` had a 60-line tail computing damage-type fallback + HP delta + publishAttackOutcome + publishPlayerDied that the resume direction would have to duplicate. The extraction is internal — no change to the resolver interface or any host-visible verb signature.

The single SDK call site is unchanged from the orchestrator's perspective: rpg-api's `submit_check_reaction.go` calls `enc.CompleteTakeAction(phasedCtx, modifiers)` regardless of direction. The SDK figures out which `applyAndPublish*` to dispatch from `attackCtx.AttackerID`.

PvP and monster-vs-monster directions return `ErrUnsupportedAttackDirection` (maps to gRPC `Unimplemented`). A future wave that wants either direction would add the corresponding `applyAndPublish*` helper to the dispatch switch; the SDK surface stays the same.

## MovementResolver (Wave 2.11e)

`MovementResolver` is the second instance of the resolver-per-verb pattern that `PhasedCombatResolver` established. It lets the encounter SDK delegate per-step movement mechanics (MovementChain execution, OA triggering) to a rulebook implementation without importing rulebook packages.

```go
type MovementResolver interface {
    ResolveStep(input MovementStepInput) (*MovementStepResult, error)
}

type MovementStepInput struct {
    EntityID encountercore.EntityID
    FromHex  encountercore.Hex
    ToHex    encountercore.Hex
}

type MovementStepResult struct {
    Prevented     bool
    PreventReason string
}
```

Triggers flow via the buffered bus subscription only — there is intentionally no resolver-returned trigger slot on the result. Chain subscribers (Disengage marker, OA condition) publish `ReactionTriggerEvent`s on the encounter bus during `ResolveStep`; the SDK installs a buffered subscriber per step to observe them. The bus path is canonical for OA/reaction handoff and matches `PhasedCombatResolver`'s shape applied to attack reactions.

The orchestrator (rpg-api) wires a resolver via `WithMovementResolver(...)`. The orchestrator's implementation wraps the rulebook's `combat.MoveEntity` so chain subscribers (Disengage marker, OpportunityAttackCondition) fire per step and OAs resolve inline via the rulebook's `triggerOpportunityAttack` → `combat.ResolveAttack` path.

### Per-step iteration vs legacy single-jump (both movement directions)

`Encounter.Move` (player direction) and `Encounter.applyNPCMovement` (NPC direction, called from `NPCAct` with `monster.TakeTurn`'s movement output) both branch on resolver presence using the same shared `iterateMovementStepsForEntity` helper:

| Mover | Caller | Resolver wired? | Path | SDK position update | Chain executes | OA fires |
|---|---|---|---|---|---|---|
| Player | `Encounter.Move` | No | Legacy single-jump | once, to `path[-1]` | never | never |
| Player | `Encounter.Move` | Yes | Per-step iteration | once, to the final hex of the traveled path | per step via resolver | inline (NPC reactor) |
| NPC | `Encounter.applyNPCMovement` | No | Legacy single-jump | once, to `path[-1]` | never | never |
| NPC | `Encounter.applyNPCMovement` | Yes | Per-step iteration | once, to the final hex of the traveled path | per step via resolver | inline (player reactor) |

Wave 2.11e #667 shipped the player-direction iteration; Wave 2.11e #668 shipped the NPC-direction mirror. Same `MovementStepInput`/`MovementStepResult` types in both directions; the SDK is direction-agnostic per #658 Q4 signoff (no `EntityType` field on the input — the resolver impl differentiates from its own lookup).

The per-step path accumulates `traveled` as it iterates; the SDK only mutates position (Player.View.Position or MonsterData.Position) once, after the loop. Step-by-step position mutation in the spatial room happens externally in the resolver impl (combat.MoveEntity calls `room.MoveEntity` per step), but the encounter SDK keeps its own position state in sync by committing once at the end.

For tests that need to drive NPC movement with a deterministic path (rather than depending on `monster.TakeTurn`'s AI output), `Encounter.MoveNPCSteps(npcID, path)` is the public seam that calls into the same iteration mechanics.

When no resolver is wired, the legacy single-jump behavior is preserved for non-combat encounters (free-roam, social). The shape was load-bearing for Wave 2.11d's verification gate: the active.md B8 probe asserted that movement without a resolver does NOT trigger OAs. The new per-step path activates only when a resolver is explicitly supplied.

### Truncated-traveled-path event publication

When chain prevention (Disengage, etc.) blocks a step mid-path, the encounter SDK stops at the previous successfully-traveled hex. The `MoveEvent` published carries only the actually-traveled segments, NOT the requested path. Same for `HexRevealedEvent` (computed from the final traveled hex) and `EntityAppeared/Disappeared` events. Wire clients see the truthful outcome, not the intent.

`applyAndPublishMove` is the helper shared between the legacy single-jump path (called with `traveledPath = requested path`) and the per-step path (called with `traveledPath = actually-moved subset`).

### Trigger buffer drain

The SDK installs a buffered subscriber on `ReactionTriggerTopic` per step. Chain subscribers (`OpportunityAttackCondition.onMovementChain`) publish `ReactionTriggerEvent`s when their predicate matches; the buffered subscriber catches them. In Wave 2.11e NPC-OA-only scope the SDK does not partition or act on captured triggers — NPC OAs are resolved inline by the resolver impl during the same `ResolveStep` call (combat.MoveEntity → triggerOpportunityAttack → ResolveAttack runs end-to-end, applying damage + publishing AttackResolved on the bus before ResolveStep returns).

The buffer infrastructure is installed for shape parity with `TakeActionPhased` and to flush subscriptions cleanly per step. The second-branch consumer (player-pause for Sentinel-shape or spell reactions) is deferred to issue #665.

### Damage application during Move iteration (#675)

OAs that fire inside `combat.MoveEntity` resolve hit + damage end-to-end (combat.ResolveAttack runs synchronously inside `triggerOpportunityAttack`) and publish `DamageReceivedEvent` on the rulebook bus before `ResolveStep` returns. But the encounter SDK owns HP state — without an explicit hand-off the events would fire on the bus, no subscriber would translate them to encounter-side state, and the goal-sentence verification "OA fires AND damage applies" would silently fail (chain works, dice roll, target's HP doesn't budge).

The fix mirrors the `applyCapturedDamage` shape used by `NPCAct` (which has the same surface — captured rulebook events post-action need encounter-side translation):

1. `iterateMovementStepsForEntity` installs a `subscribeDamage` buffer on the encounter bus per step, alongside the `ReactionTriggerTopic` buffer. The defer chain inside the inner step closure tears both buffers down on return — even on resolver panic.
2. After `ResolveStep` returns, the SDK reads the captured `DamageReceivedEvent` slice and dispatches each through `applyMoveDamage`.
3. `applyMoveDamage` mirrors `applyCapturedDamage` but resolves source position dynamically (`findEntityPosition`) because Move-path OAs fire from EITHER direction (player attacker on a fleeing NPC, or NPC attacker on a fleeing player) — the per-viewer LoS projection key varies by attacker type. The HP delta + encounter-side `DamageDealtEvent` + kill-chain on `>0 → 0` transition all go through the same code path as the NPCAct equivalents.

The capture/apply happens BEFORE the `Prevented` check: OAs fire whether or not the chain ends up preventing the step, so damage applies either way.

### Subscriber-window scoping in NPCAct (#677)

`NPCAct` owns two windows that publish `DamageReceivedEvent` on the encounter bus: the **movement window** (`applyNPCMovement` → `iterateMovementStepsForEntity` → resolver fires OAs that go through `combat.ResolveAttack`) and the **attack-resolution window** (`applyCapturedAttacks` → `combat.ResolveAttack`). Each window has its own subscriber installed at the right scope:

- **Inner per-step** subscriber (from #675) owns the movement window. It applies HP delta + publishes encounter-side events via `applyMoveDamage` before the next step runs.
- **Outer** subscriber (the original `subscribeDamage` at NPCAct setup) owns the attack-resolution window. It applies HP delta via `applyCapturedDamage` after `applyCapturedAttacks` returns.

The outer is installed AFTER `applyNPCMovement` returns, not at NPCAct entry. If it were installed at entry, both subscribers would fire on the same movement-OA `DamageReceivedEvent` — `applyMoveDamage` would apply HP during iteration, then `applyCapturedDamage` would apply the same delta again after movement returns. A 7-HP goblin hit by a 5-damage OA would land at -3 HP (clamped to 0) and the kill chain would fire twice. Wave-2.11e-goal-blocking double-apply bug — caught by director review of #677.

Tests cover the production path explicitly (`TestNPCAct_MovementOA_AppliesDamageOnce` in `npc_test.go`). The earlier movement tests in `move_resolver_test.go` exercise `MoveNPCSteps` directly, which bypasses NPCAct's outer subscriber entirely — useful for walker-level coverage, but not a substitute for the production-path regression guard.

### Scope deferred (#665)

When a player-bearer reaction becomes a goal-shaped feature (Sentinel feat, Shield/Counterspell, etc.), the per-step iteration loop gains a second branch: partition triggers by reactor type, persist `PendingReactionPrompt` for player-bearer triggers, publish a sentinel `errPlayerPausedForReactionDuringMove`, and resume via the existing `SubmitCheck{take_reaction}` path. The design sketch lives on #665; the structural seam is already in place (the per-step iteration + buffer drain are the load-bearing infrastructure).

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
