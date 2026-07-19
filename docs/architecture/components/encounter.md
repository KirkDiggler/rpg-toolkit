---
name: encounter module
description: Orchestrator-facing SDK for running an encounter end-to-end — sealed event taxonomy, process-scoped Broker, transient Encounter aggregate, combatant hydration cascade, discrete-phase combat orchestration, MovementResolver seam (both movement directions), walled-room space + wall-aware LoS + inline combat entry
updated: 2026-07-13
confidence: high — #689 made LoadFromData own combatant hydration (the #684 double-subscribe cure); Wave 2.11d shipped discrete-phase combat; Wave 2.11e extended CompleteTakeAction to accept either PvE attack direction AND added MovementResolver for per-step movement in BOTH directions; #697 (TakeAction wave, ADR-0032) deleted the attack-only gate — non-attack refs delegate to the held character's economy/menu, turn-start seeding moved into the engine, ActorTurnState exposes the menu as data; #757 (the walled room) added SpaceData, wall-aware VisibleHexesAt/CanSeeAt, wall-blocked movement, and inline combat-entry self-transition
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

- `encounter/core` — identity primitives (`EncounterID`, `PlayerID`, `EntityID`) and spatial primitives (`Hex`, `HexSet`). Exists to break the encounter↔events package import cycle. `HexSet` has custom `MarshalJSON`/`UnmarshalJSON` (struct map keys can't serialize via the default codec). Since #757, `core` also depends on `tools/spatial`: `Hex.ToCube`/`HexFromCube` (field-rename bridge to `spatial.CubeCoordinate`) and `Hex.ToPosition`/`HexFromPosition` (offset-coordinate bridge to `spatial.Position`, pointy-top orientation) — see "Walled rooms" below.
- `encounter/events` — sealed `EncounterEvent` interface, three concrete events (`MoveEvent`, `HexRevealedEvent`, `DoorOpenedEvent`), and `AudienceSet` (event-routing concept; lives with events).
- `encounter/events` — sealed `EncounterEvent` interface (AWS v2 SDK marker pattern: unexported `isEncounterEvent()` makes the interface externally unsatisfiable). Concrete events implemented in slice 1: `MoveEvent`, `HexRevealedEvent`, `DoorOpenedEvent`. Each has its own `MarshalJSON`/`UnmarshalJSON` so unexported `encID`/`seq` fields round-trip without leaking construction-only state.
- `encounter/perception` — pure projection functions (`ProjectMove`, `ProjectDoorOpen`) and `View` value type. `VisibleHexesAt`/`CanSeeAt` take a `room spatial.Room` parameter since #757 — nil room is the original pure-radius stub (unchanged for encounters with no `SpaceData`); a non-nil room additionally excludes hexes `room.IsLineOfSightBlocked` reports as wall-blocked. SightRange always caps distance first, regardless of room.
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
- **Hydration alone satisfies the combatant gate (#750).** `isPlayerCombatant`
  (now an `*Encounter` method, `combat.go`) treats a seat as combat-ready when
  it is ACTUALLY HELD (`e.heldCharacter(...) != nil`), not just when the flat
  `AC`/`DamageDice` snapshot is set — a hydrated resolver ignores the flat
  snapshot entirely (per the bullet above), so requiring it too would strand
  any host that hydrates real characters but has no honest value to offer for
  a precomputed attack-bonus/damage-dice field (e.g. rpg-api's lobby
  `StartEncounter`, which seeds real HP/AC but leaves those three fields
  zero-value on purpose). Deliberately checks the held-character map, not
  `len(DataJSON) > 0` on the input: DataJSON being set means a seat carries
  rehydratable data, not that hydration happened — `New()`+`AddPlayer` never
  hydrate, only a `LoadFromData` round-trip's cascade does.
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
| monster→player | `data.Monsters` | `applyAndPublishNPCOutcome(monster, player, outcome)` | `publishPlayerDied` (still no removal from initiative — "seats stay on death" is a deliberate, standing call — but now DOES check encounter-end: rpg-toolkit#772/#782, see below) |
| player→player | n/a | rejected with `ErrUnsupportedAttackDirection` | n/a |
| monster→monster | n/a | rejected with `ErrUnsupportedAttackDirection` | n/a |

`applyAndPublishNPCOutcome` is extracted from the per-attack body of `applyCapturedAttacks` (encounter/npc.go) and re-used from both the inline NPC turn and the Shield-resume direction so the two paths emit identical event shapes. Before this extraction, the inline `applyCapturedAttacks` had a 60-line tail computing damage-type fallback + HP delta + publishAttackOutcome + publishPlayerDied that the resume direction would have to duplicate. The extraction is internal — no change to the resolver interface or any host-visible verb signature.

The single SDK call site is unchanged from the orchestrator's perspective: rpg-api's `submit_check_reaction.go` calls `enc.CompleteTakeAction(phasedCtx, modifiers)` regardless of direction. The SDK figures out which `applyAndPublish*` to dispatch from `attackCtx.AttackerID`.

PvP and monster-vs-monster directions return `ErrUnsupportedAttackDirection` (maps to gRPC `Unimplemented`). A future wave that wants either direction would add the corresponding `applyAndPublish*` helper to the dispatch switch; the SDK surface stays the same.

### Encounter-end predicate: TPK (rpg-toolkit#772, #782)

`checkEncounterEnd` (death.go) now has two predicates, checked in order:

1. `len(data.Monsters) == 0` (all hostiles defeated) — victory, `Reason = "all_hostiles_defeated"` (Wave 2.10).
2. `allPlayersDead()` (every seated `PlayerData` has `Dead == true`) — defeat/TPK, `Reason = "tpk"` (rpg-toolkit#772/#782).

Before this, player death never fed `checkEncounterEnd` at all — only `killEntity` (the monster-kill path) called it. A solo PC's confirmed death left the encounter running forever, looping the corpse's turns (`#772`, confirmed live 2026-07-18). `publishPlayerDied` is now the single chokepoint both player-death paths funnel through (the `CharacterDied` rulebook bridge for 3-failed-death-saves, and `applyUnconsciousOnZeroHP`'s non-hydrated instant-death fallback for flat-snapshot seats) — it marks `PlayerData.Dead = true`, then calls `checkEncounterEnd()`, so every current and future death path gets TPK-checked with no call site able to forget it.

**`Dead` means CONFIRMED death, not merely unconscious.** A player at HP<=0 who is still death-saving (could nat-20 revive) has `Dead == false` — only 3 failed saves, or the instant-death fallback for a seat with no hydrated character, sets it. This preserves the death-save mechanic's chance to matter even for the last conscious player: ending the encounter the instant everyone hits 0 HP would short-circuit a possible revival.

**No wire/proto change.** `EncounterEndedEvent.Reason` was already a bare string (`rpg-api-protos`' `EncounterEnded{reason}` message, and rpg-api's translator maps it field-for-field with no reason allowlist) and `"tpk"` was already pre-declared as a future value in this event's own doc comment alongside `"fled"`/`"negotiated"`/`"time_out"`.

**Reentrancy hazard this surfaced.** Wiring TPK detection into the death-save auto-roll (which fires at turn start) means `checkEncounterEnd` can now run *synchronously inside* `seedActorTurn`'s own `TurnStartTopic` publish — a fatal 3rd failure at turn start can end the encounter before `seedActorTurn` finishes. All three `seedActorTurn` call sites now check `Mode == ModeEnded` immediately after the publish returns and bail out if so:

- `EndTurn` (combat.go) — otherwise it would go on to publish `TurnStartedEvent`/`TurnStateChangedEvent` for a turn that will never be played, carrying `Round`/menu state `checkEncounterEnd` already zeroed.
- `SetMode`'s first-actor seed (combat.go) — this one was also a latent **panic**: it indexed `e.data.Initiative[0]` *after* the seed call, on a slice `checkEncounterEnd` can null out. Fixed by capturing the entity id into a local variable before seeding.
- `seedActiveActorIfUnseeded`'s LoadFromData catch-up (turn_economy.go) — same shape, lower likelihood (only fires for a character's first-ever seed this session).

Without the guard inside `seedActorTurn` itself (turn_economy.go, right after the `TurnStartTopic` publish, before the downed-actor HP check), the function would continue past the encounter's end and call `char.StartTurn` — reseeding a FULL economy for an actor whose encounter just concluded, directly undoing `checkEncounterEnd`'s own `ExitCombat` cleanup moments after it ran (`e.data.Round` reads as `0`, just cleared, giving away that this reseed happened post-transition).

**Unconscious economy hole, two independent windows (rpg-toolkit#781).**

*Window 1 — mid-turn.* A player who goes unconscious *during their own turn* (e.g. an opportunity attack triggered by their own `Move`) kept their already-seeded full economy — nothing re-checked it before their next `TakeAction` call, so they could still land an attack "while unconscious." `applyUnconsciousOnZeroHP` (death.go) checks `e.ActiveActor() == target.EntityID` after applying the Unconscious condition and, if true, immediately zeroes the rest of the turn via `char.EndTurn` — the same call `seedActorTurn`'s own downed-at-turn-start branch already uses. `Move`'s movement-spend accounting (encounter.go) is adjusted to match: it used to assume the post-pre-check spend was "structurally guaranteed to succeed," which this fix breaks for a mid-move knockdown (zeroing `MovementRemaining` too) — `Move` now skips the spend when the mover's `PlayerData.HP` reads 0 by the time the per-step iteration returns, rather than surfacing a misleading `ErrInsufficientMovement` over a move that demonstrably already happened.

*Window 2 — every subsequent turn (rpg-toolkit#784, closed in the same PR after a gate review caught the false premise).* The above alone was believed sufficient because "turn-start seeding already zeroes a downed player's economy correctly" — true of `seedActorTurn`'s *gating logic* (rpg-toolkit#733), false of the *data it reads* for any player downed by real combat. `seedActorTurn`'s downed-actor check reads `char.GetHitPoints()`, but neither `applyAndPublishNPCOutcome` (combat_phased.go, NPC-inflicted damage) nor `applyCapturedDamage` (npc.go, Move-OA damage) ever called `char.ApplyDamage` on the hydrated defender — only `PlayerData.HP` got mutated. Confirmed by grep: `Combatant.ApplyDamage` has exactly one caller anywhere in the rulebook (`combat.DealDamage`, damage.go), itself uncalled anywhere — dead code; neither the legacy `ResolveAttack` nor the phased `ResolveAttackHit`/`ApplyAttackOutcome` (what these two call sites actually dispatch through) ever touch a hydrated defender's HP. So a combat-downed player's held character kept reporting stale pre-damage HP forever, and `seedActorTurn` re-seeded a FULL 1/1/1/30 economy on *every* turn after the knockdown, not just the mid-turn one Window 1 covers — the actual shape of "landed a killing blow while unconscious" and "full economy one round, zero another." The pre-existing `turn_economy_downed_test.go` coverage never caught this because every fixture in that file synthesized `HitPoints:0` directly into `DataJSON` rather than downing a player via combat — a shape combat damage never actually produces.

Fix: `applyUnconsciousOnZeroHP` does a one-time HP correction, right before the Unconscious condition is applied (the single existing chokepoint every player's `>0`→`0` transition already funnels through) — reads `char.GetHitPoints()`'s own stale value and applies exactly that much back to itself via `char.ApplyDamage`, landing at true 0. No need to know the real attack's damage number; the target is already authoritatively down, this just makes the held character agree. Two things verified before landing this: (1) the nat-20 revival window (`seedActorTurn` reads HP *after* the synchronous `TurnStartTopic` publish specifically so a nat-20's `HealingReceivedEvent` is visible first) is not regressed — `onHealingReceived`'s `+1` now lands on a truly-zero base instead of a stale nonzero one, a strict correctness improvement, not a behavior change to the revival mechanic itself; (2) no double-counting against resistance — `DamageInstance.Amount`'s own doc comment states "after modifiers, before resistance," and `char.ApplyDamage`'s body is confirmed to be pure `c.hitPoints -= total` with no resistance/vulnerability lookup, so a raging barbarian's physical resistance (applied upstream, in the damage chain's `StageFinal`) cannot soften this correction.

**#784 remains open, re-scoped.** This closes only the knockdown-transition case. Damage that does NOT down a player (20 HP → 5 HP) still leaves `char.hitPoints` stale until a knockdown or a `LoadFromData` reload — general per-hit sync is unaddressed. The monster side (does `MonsterData`/`monster.Monster` have the same gap?) is unexplored and also left open.

New regression coverage: `TestDownedPlayerViaRealCombat_StaysZeroedAcrossMultipleTurns` (turn_economy_downed_test.go) knocks a player down via a real `NPCAct` hit — no synthesized HP anywhere in the fixture — then checks zero economy AND a rejected `TakeAction` across two subsequent turn-starts, the exact shape none of the file's other (synthesized-HP) tests could exercise.

**A second, opposite-direction gap found live-testing this wave: `PlayerData.HP` never synced FROM healing.** A live playtest reported TPK blocked entirely (seven unconscious turns, no death-save progress observed) — reproduced directly against a locally-built rpg-api with local replaces to this branch (devseed `wave-3-barbarian` + grpcurl `EndTurn` cycling, no browser). Three independent live reproductions and a dedicated toolkit-level per-RPC-reload regression test (`TestDeathSaveRoll_SurvivesPerRPCReload_ViaRealEndTurn`, unconscious_zero_hp_test.go) all showed `onTurnStart`'s roll firing correctly and repeatedly across reloads — the auto-roll mechanism itself was never broken. Building that regression test surfaced a *different*, real bug: a nat-20 death-save (`RegainedConsciousness`) correctly revives the character and removes the Unconscious condition server-side, but nothing ever synced `PlayerData.HP` back up to match — `character.onHealingReceived` only ever touched `c.hitPoints`. A revived player's snapshot (and anything reading it, a reconnecting client's view included) stayed stuck at 0 HP / "still unconscious" forever, even though the character had genuinely revived. This is the opposite direction of #784's still-open gap (damage not syncing IN), not the same bug.

Fix: `subscribeHealingReceivedBridge` (death.go), a new permanent bridge (same lifetime shape as `subscribeCharacterDiedBridge` et al. — healing can fire from any future turn, not just once per verb call) subscribed to the rulebook's `HealingReceivedTopic`, applying the event's own `Amount` directly to `PlayerData.HP` (clamped to `MaxHP`) whenever it resolves to a seated player. Deliberately NOT "read `char.GetHitPoints()` and trust it wholesale at `ToData()` time" — that was tried first and reverted after it broke `TestAliveActivePlayer_UnaffectedByFix` (midturn_unconscious_economy_test.go): `char.GetHitPoints()` is *stale-low* for #784's still-open per-hit gap, so unconditionally trusting it at sync time silently overwrote a correct, freshly-damaged `PlayerData.HP` with a stale pre-damage value on every `ToData()` call. Reacting to the healing event itself, with a targeted delta, can't make that mistake — it only ever fires on a genuine heal.

**Known note, not fixed here:** a flat stat-snapshot (non-hydrated) player's instant death (the `applyUnconsciousOnZeroHP` `char == nil` fallback) can fire TPK synchronously mid-verb — e.g. partway through a batch of captured Move-OA damage instances, or mid-`NPCAct`. The calling function can continue past that point and publish further attack/move events describing activity for an encounter that has already terminally ended (the same reentrancy class the `seedActorTurn` guards above close for the turn-start path, but for a different, mid-verb call shape). Lower severity, deliberately out of this wave's scope.

### Encounter-end condition sweep (rpg-toolkit#752) + action-economy clear (rpg-toolkit#767)

`checkEncounterEnd` (death.go) is the single chokepoint every encounter-end predicate feeds through (victory or TPK — see above). Before it publishes the terminal `EncounterEndedEvent`, it calls `endCombatForPlayers`, which — for every held player character — publishes `dnd5eEvents.CombatEndEvent` (via `character.Character.EndCombat`) and calls `character.Character.ExitCombat`. This runs identically regardless of which predicate fired — no reason-specific special-casing — so a TPK gets the same condition sweep and economy clear a victory always did.

**Raging is not reachable as "still active" at a TPK ending, discovered writing the #782 composition test.** `RagingCondition.onConditionApplied` unconditionally self-removes rage the instant `Unconscious` is applied to the same character (RAW-correct) — which happens at knockdown, not at combat's end. Every hydrated character's HP-zero transition routes through Unconscious first (#733), so a character who is about to die from a TPK-ending death-save chain always has rage stripped several turns before death. Raging is also the only condition in this rulebook that subscribes to `CombatEndTopic` — so for a TPK specifically, there is nothing left for the sweep to remove; `ExitCombat`'s economy clear is the verifiable half of the guarantee (`encounter/tpk_test.go`'s `TestTPK_RunsEndOfCombatSweep_ExitCombatClearsEconomy`).

**Condition sweep (`EndCombat`, #752) exists because combat-scoped conditions (rage being the motivating case) previously had no way to hear "the encounter is over" — `RagingCondition` only self-removed on no-combat-activity-this-turn, duration expiry, unconsciousness, or rest. A raging character whose *own killing blow* ended the fight skipped all of those triggers, so the condition rode all the way to `ToData()` still present in the persisted `character.Data.Conditions`, and the next encounter's `LoadFromData` cascade silently re-`Apply`'d it — granting the rage damage bonus to a character who never activated rage in that encounter and had no charges left.

**Lifetime model: opt-in per condition, not a taxonomy on Encounter.** `CombatEndTopic` is just another self-termination trigger a condition can subscribe to in its own `Apply`, the same shape `RestTopic` already established (`RagingCondition.onRest` / `onCombatEnd` sit side by side). A condition that should outlive combat (a curse, say) simply never subscribes — the encounter package has no allowlist or type-switch over condition kinds, and doesn't need one to add the next combat-scoped condition.

**Action-economy clear (`ExitCombat`, #767) exists because `ActionEconomy` — a flat, encounter-unscoped field on the persisted character (`rulebooks/dnd5e/character/data.go`) — has no lifetime management of its own: `ToData()`/`LoadFromData` round-trip whatever is in it verbatim, with no encounter-identity check. `ExitCombat` (`character.Character`) has always existed for exactly this ("Call this when the encounter ends") but nothing ever called it — not rpg-api, not the toolkit's own encounter package. A character who finished a fight carried their depleted economy (e.g. `movement_remaining: 0`) into every SUBSEQUENT encounter forever — found live via Redis inspection in The Dungeon wave 1's closing playtest (rpg-api PR #645, commit 759eca9). That commit added a defensive `ExitCombat` + persist call at rpg-api's `StartEncounter` as a backstop, explicitly NOT the fix; this PR is the fix. At the time this landed, the backstop still mattered for a TPK specifically (`checkEncounterEnd`'s only predicate was `len(data.Monsters)==0`, so it never fired on a TPK) — closed by rpg-toolkit#772/#782 (see the predicate section above), which added the `allPlayersDead` predicate. The rpg-api-side backstop itself stays in place regardless: it also covers abandoned/disconnected encounters that never reach ANY toolkit-side end predicate (no one ever confirms a kill or a death), which is outside what any encounter-end predicate can detect.

Flat stat-snapshot seats (no hydrated `*character.Character`) have nothing to sweep and are skipped for both calls. Ordering: the sweep runs *before* `EncounterEndedEvent` publishes, so any `StatusRemoved` a condition emits is sequenced ahead of the terminal event on the broker stream; `ExitCombat`'s economy clear carries no wire event of its own (it only affects what `ToData()` persists), so it has no ordering constraint relative to the terminal event.

**Deferred, not part of this fix (rpg-toolkit#773):** scoping `ActionEconomy` to an encounter ID (defense-in-depth against a *future* encounter-end path forgetting to call `ExitCombat`, the same way this bug happened) requires threading encounter identity into `character.ActionEconomyData` — a real change to the character/encounter boundary that deserves its own design pass. Not bundled here.

### Cross-encounter recovery: arcade recovery (rpg-toolkit#785)

**Death is encounter-scoped, not a persistent character state** (Kirk's decision, 2026-07-19, #785). Before this, a character record persisted at 0 HP after a TPK entered its *next* encounter unable to act — no death saves rolling, no recovery path, effectively bricked, since nothing in the toolkit ever restored a downed character across an encounter boundary.

`character.RestoreForNewEncounter(d *Data) bool` (`rulebooks/dnd5e/character/arcade_recovery.go`) is a no-op above 0 HP. At or below 0 HP it restores `HitPoints` to `MaxHitPoints`, clears `DeathSaveState`, and strips any `Unconscious` condition out of `Data.Conditions` by ref (peeking each blob's leading `Ref` field, the same pattern `activeConditionRefs` above uses — no full `conditions.LoadJSON` deserialization needed, since this only needs identity).

**Where it fires — and why nowhere else may call it.** `Encounter.AddPlayer` is the only call site: it's the sole place a brand-new `PlayerData` seat is created, structurally guarded to fire at most once per player per encounter (the existing "already in encounter" check). `LoadFromData` — the per-RPC rehydration cascade this doc's hydration section above describes, which reloads an EXISTING seat on every subsequent RPC against the same encounter — never calls `AddPlayer` and therefore can never restore. Resuming an encounter is not re-seating one; `RestoreForNewEncounter`'s own doc comment states this contract directly for any future caller.

**Why the Unconscious strip is load-bearing, not decorative — ties directly into the sweep two sections above.** The #752/#767 end-of-combat sweep (`endCombatForPlayers` → `char.EndCombat` → `CombatEndTopic`) never reaches the Unconscious condition: as established above, Raging is the *only* condition in this rulebook that subscribes to `CombatEndTopic`. `conditions.UnconsciousCondition` does not. So a TPK's `Data.Conditions` genuinely still carries the Unconscious blob — with its own embedded `Successes`/`Failures`/`Dead` — with nothing upstream ever removing it, even after #772/#782 wired TPK into `checkEncounterEnd`. Left alone, the next encounter's `LoadFromData` cascade would re-`Apply()` it onto a character now at full HP. `RestoreForNewEncounter`'s strip is the only thing in the toolkit that clears it.

**HP-snapshot sync.** `AddPlayer` also overwrites `PlayerData.HP`/`MaxHP` to match whenever the restore actually fires (`RestoreForNewEncounter`'s `bool` return signals this). This encounter-level snapshot and the embedded `character.Data.HitPoints` are the same two representations the #781/#784 HP-sync discussion above documents as an active divergence-bug class — restoring only the embedded value while leaving a caller-supplied `HP:0` snapshot in place would have produced a third, differently-shaped incoherent seat instead of fixing the reported one.

**Scope, deliberately not decided silently:** HP and death-save state only. Ability uses, class resources, spell slots, and hit dice are untouched — this is recovery from death/incapacitation, not a free rest, which stays a separate, real system for later.

**`Data.DeathSaveState` is dead API surface, flagged not fixed here.** Grepping the whole repo turned up zero production callers of `Character.MakeDeathSave`/`TakeDamageWhileUnconscious` (the only methods that read/write this field) outside the character package itself. The real, live death-save state lives inside the `UnconsciousCondition` instance embedded in `Data.Conditions`, auto-rolled at turn start. `RestoreForNewEncounter` still clears `Data.DeathSaveState` for API honesty, but the load-bearing half of this fix is the condition strip above, not this field. A future cleanup candidate, not acted on here.

### Snapshot-visible active conditions (rpg-toolkit#754, #778)

Previously, encounter snapshots carried no projection of a held entity's active conditions — a client that (re)connected mid-encounter only ever learned about a condition from the live `StatusApplied`/`StatusRemoved` broker stream, never from state. A condition hydrated in via `LoadFromData` (as opposed to applied while a client was connected and watching) was mechanically active but invisible — confirmed during the #752 fix (the leaked raging condition added a `raging:2` damage component with no 🔥 badge anywhere for any viewer).

`PlayerData.ActiveConditions` / `MonsterData.ActiveConditions` (`[]string`, ref strings like `"dnd5e:conditions:raging"`) now carry this. Populated by `syncCombatantsToData` (hydration.go, the same write-back cascade `ToData()` already runs for `DataJSON`) via `activeConditionRefs` (active_conditions.go), which reads the ALREADY-serialized `character.Data.Conditions` / `monster.Data.Conditions` blobs `ToData()` just produced — not a second pass over `GetConditions()`+`ToJSON()`, which would pay the same per-condition serialization cost twice per combatant per RPC (Copilot + gate review on PR #776 both flagged the original draft's double-serialization). Safe to read post-load: entities reaching `syncCombatantsToData` were hydrated via the encounter's `LoadFromData` cascade, so `monster.Monster.traitData` (the pre-bus staging slice only reachable from factory construction) is always empty by the time `ToData()` runs here — `Data.Conditions` is exactly `serialize(GetConditions())`, no more and no less. `activeConditionRefs` reads the `Ref *core.Ref` field every condition Data type in this rulebook leads with (`RagingData`, `DisengagingData`, `UnconsciousData`, the monstertraits condition types, etc. — verified across the package, not assumed). No rulebook-specific type-switch, matching the toolkit's existing rulebook-agnostic dispatch elsewhere.

Deliberately minimal: ref only, no duration/source/display data — mirrors `ConditionAppliedEvent`'s own cause-event shape (`translateConditionAppliedEvent` already enriches `display_name`/`icon_hint` on the rpg-api side for the live path; the snapshot path is the same division of labor). A condition that fails to serialize, or whose JSON omits `ref`, is skipped rather than failing the whole snapshot (best-effort visibility, mirrors `monster.Monster.ToData`'s own "skip conditions that can't be serialized" precedent). Nil (omitted from the wire), not an empty slice, when nothing qualifies.

**Monsters are included, not just players** — "monster conditions" in this rulebook are `monstertraits` (Immunity/Vulnerability/PackTactics/UndeadFortitude) alongside anything applied mid-fight; both become genuinely-`Apply`'d `ConditionBehavior` instances once a monster has been through any `LoadFromData` cycle (`AddTraitData`'s pre-bus staging only matters before the first such cycle — verified by reading `monstertraits.LoadMonsterConditions`, which routes every entry uniformly through `LoadJSON` + `Apply` + `AddCondition`, with no distinction preserved from the persisted blob's origin).

**This is a toolkit-side data-shape fix only — the wire is not yet wired.** The proto `Entity.status_effects` field (`dnd5e/api/v1alpha2/encounter/types.proto`) already exists for exactly this (`repeated StatusEffect status_effects = 6; // visible only`) — no proto change needed. But rpg-api's snapshot-building code (`ProjectFor` / the `entityForID`/`playerEntity`/`monsterEntity` builders) never reads `ActiveConditions` and never populates `status_effects` from anything (confirmed: `StatusEffect{}` is constructed in exactly one place, `translateConditionAppliedEvent`, the live per-event path — never in the snapshot path). Until rpg-api adds that projection, `ActiveConditions` exists on the toolkit's `Data` and is provable at the `ToData()` level (see `active_conditions_test.go`), but does not yet reach a reconnecting client's screen. Tracked as rpg-api#651.

#### Excluding build-time-granted conditions (rpg-toolkit#778)

`ActiveConditions`' original (#754) shape was the *full* `GetConditions()` set with no filtering — which mixes two structurally different kinds of condition. `Raging` is runtime-attached: activated via `Encounter.ActivateFeature`, the encounter package's ONLY bridge from a dnd5e-level `ConditionAppliedEvent` to a broker `events.ConditionAppliedEvent`, so it genuinely IS announced on the live stream. `MartialArts`/`UnarmoredDefense` (Monk/Barbarian Grant.Conditions) and monster traits (Immunity/PackTactics/etc.) are build-time-granted: attached once, permanently, and never live-announced. Without filtering, every Monk's snapshot would carry a permanent "MartialArts" badge forever, and every goblin a permanent "PackTactics" badge — the same disease as the leaked-rage bug #754 fixed, just moved one level down (found by the review gate on PR #776, finding #1: "ActiveConditions is a strict superset of what the live stream ever announces").

**Both attachment paths go through the identical pipeline.** `Draft.Finalize` (character creation) and `Encounter.ActivateFeature` (live combat) both publish a dnd5e-level `ConditionAppliedEvent`; `Character.onConditionApplied` (character.go, installed by `subscribeToEvents`) is the ONLY code that appends to `c.conditions`, for both paths uniformly — there is no code-level branch distinguishing origin at attachment time. The only difference is WHEN and on WHICH bus the publish happens: `Draft.Finalize` runs at character creation, before the character belongs to any encounter, on a bus no encounter-level bridge subscriber will ever be attached to. `Encounter.ActivateFeature` installs its capture subscriber in a narrow, call-scoped window (subscribe, call `ActivateAbility`, capture, unsubscribe) that Finalize's publish can never fall inside. So build-time-granted conditions are structurally unobservable via the live path by construction, not because of any flag.

**No per-instance runtime provenance marker needed — verified by call graph, not just current data.** `conditions.CreateFromRef` (the factory `compileConditions` calls to build every `Grant.Conditions` entry into a live `ConditionBehavior`) has exactly ONE caller in this rulebook: `compileConditions` itself. Every genuinely live-activated condition (e.g. `RagingCondition`) is constructed directly by its own feature's activation code, never through `CreateFromRef`. Attachment mechanism therefore correlates 1:1 with condition ref identity today — a STATIC, ref-keyed exclusion set is sufficient; the issue's two proposed shapes ((a) dynamic per-instance filtering, (b) an exported provenance marker) both turned out to be more than what's needed.

`character.StructurallyPermanentConditionRefs()` (`rulebooks/dnd5e/character/permanent_conditions.go`) derives this set from the rulebook's own authored data — walking `classes.ClassData` + `classes.GetGrants` for every `Grant.Conditions` entry at any level, plus `fightingstyles.All()` mapped to its condition ref — rather than a hand-maintained literal, so a newly migrated class or fighting style is picked up automatically instead of silently missing (the exact "someone forgot to update a list" failure mode #767 was about, applied here). `monstertraits.AllTraitRefs()` (`loader.go`) is the monster-side equivalent, mirroring `LoadJSON`'s dispatch switch — this one IS a hand-maintained mirror (no existing enumerable trait registry to derive from without a larger refactor to `LoadJSON` itself, judged out of scope for this issue), documented as such in its own doc comment. `encounter/active_conditions.go` computes the union once at package init and filters `activeConditionRefs` against it.

**Golden-list regression test, not just unit coverage** (`encounter/permanent_conditions_test.go`): pins the exact derived set (9 character-side refs + 4 monster-side refs) so a future class/style/trait addition changes the test's expected list, forcing a human to confirm the new ref really is build-time-only before the test goes green again — the tripwire for the risk the call-graph verification only covers for *today's* rulebook content, not future additions that could reintroduce a ref reachable through both paths.

**Structural invariant this depends on, documented on `StructurallyPermanentConditionRefs`'s own doc comment**: if a future condition becomes reachable through both a `Grant.Conditions` entry and a live `ActivateFeature` path, this derivation would incorrectly exclude it from `ActiveConditions` even on the encounters where it WAS genuinely live-activated. The fix at that point is a real per-instance provenance marker (option (b)) — not more static-set patching.

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

## Walled rooms, wall-aware LoS, and inline combat entry (rpg-toolkit#757)

Wave 1 of "the walled room" bridges the encounter SDK to `tools/spatial` +
`tools/environments` — the heavy machinery (wall LoS, movement blocking,
serialization, spawn engine) already existed in those modules; this wave
connects it. Design doc: `rpg-project/ideas/the-dungeon/design.md`.

### SpaceData: snapshot, not seed-regeneration

`Data.Space *SpaceData` (`Walls []environments.WallSegmentData`, `Width`,
`Height`) persists a room as a **snapshot**, not a regeneration seed.
`DoorData` already persists mutable state directly, and destroyed walls /
opened doors (wave 2+) can't replay from a seed either — picking the
snapshot representation now avoids a representation split later. (Before
rpg-toolkit#787, `QuickRoom`'s 3-arg convenience wrapper never called
`WithRandomSeed`, so `RandomPattern` was *accidentally* deterministic —
`rand.NewSource(0)` on every call, meaning every encounter shipped the same
wall layout. That's fixed now: `Build()` entropy-seeds `RandomSeed`
whenever a caller leaves it unset. Irrelevant to the snapshot-vs-seed
design choice either way — SpaceData was never going to replay from a seed
regardless of whether generation happened to be deterministic.)

`Encounter.InitRoom(width, height, pattern, seed ...int64)` builds a room
via `environments.QuickRoom`, and `LoadFromData` rebuilds one from
`Data.Space` on every call (`rebuildRoomFromData`, transient — reconstructed
each load, like `e.bus`/`e.combatants`, never serialized). Both are
nil-safe: an encounter that never calls `InitRoom` (every pre-#757 fixture)
has `e.room == nil`, and every room-aware call site in this package checks
for that — LoS falls back to pure radius, movement is unblocked. The
trailing `seed` is optional and threads straight through to `QuickRoom`
(only the first value is used) — omit it for real gameplay's entropy
default, pass one for devseed fixtures / regression tests that want the
same room every run.

**Position precision gotcha (why InitRoom doesn't use QuickRoom's room
directly):** `environments`' wall generator (`generateRandomWall`,
`wall_patterns.go`) places walls at **continuous** float positions (e.g.
`X=3.7`) — it was not built assuming hex-cell-snapped placement. Every
LoS/movement check in this package queries at the **integer** positions
`core.Hex.ToPosition()` produces. Those two essentially never collide in a
`spatial.Room`'s position-keyed occupancy map, which would make most
generated walls silently non-blocking. `InitRoom` avoids this by never
registering `QuickRoom`'s own room as `e.room` — it snapshots the walls
(rounding each wall entity's position to the nearest integer hex cell:
`space.go`'s `snapshotWalls`) and calls the same `rebuildRoomFromData` path
`LoadFromData` uses, so `e.room`'s walls always sit at exact integer hex
positions. One degenerate (`Start == End`) `WallSegmentData` entry per
discretized wall hex — wave 1 needs per-hex blocking, not polyline
geometry; per-viewer wall reveal (which would want real geometry) is a
wave-2+ concern.

### Hex ↔ CubeCoordinate ↔ Position bridge

`encounter/core`'s `Hex{Q,R,S}` and `spatial.CubeCoordinate{X,Y,Z}` are the
same cube math with different field names — `Hex.ToCube`/`HexFromCube` is a
pure rename, no computation. `spatial.Position{X,Y float64}` is a
**different** representation (hex-grid offset coordinates, not cube) that
`spatial.Room`'s LoS/movement methods actually take; `Hex.ToPosition`/
`HexFromPosition` compose the cube bridge with `spatial`'s existing
`ToOffsetCoordinateWithOrientation`/`OffsetCoordinateToCubeWithOrientation`,
hardcoded to `HexOrientationPointyTop` (the only orientation
`environments.QuickRoom`'s room builder constructs — D&D 5e standard).
`npc.go`'s three pre-existing inline `spatial.CubeCoordinate{X: h.Q, ...}`
conversions were refactored onto `Hex.ToCube`/`HexFromCube` in the same
change — one bridge, not four copies of the same field mapping.

### Wall-blocked movement

`Encounter.truncateAtWall(path)` (space.go) returns the prefix of a
requested path up to (not including) the first hex `room.CanPlaceEntity`
rejects — checked via a throwaway `wallCheckEntity` (players/monsters are
never themselves placed into the spatial room; only walls occupy it, so
`CanPlaceEntity`'s occupancy check only ever finds walls). Applied at the
top of both `Move` (player direction) and `applyNPCMovementSteps` (NPC
direction, shared by `NPCAct` and the `MoveNPCSteps` test seam) — before
any movement-budget/resolver logic sees the path, mirroring how a
resolver's chain-prevented truncation already works. A fully-blocked first
hex is a no-op (nil error, no state change, no events), matching the
resolver's own "prevented at first step" semantics.

### Inline combat-entry self-transition

`checkCombatEntry` (combat.go) mirrors `checkEncounterEnd`'s self-transition
at combat's *other* edge (death.go): when the encounter is `ModeFreeRoam`
and any player has LoS (`perception.CanSeeAt`, wall-aware) to any monster,
it calls `SetMode(ModeTurnBased)` — the exact same initiative-roll +
`ModeChanged`/`InitiativeRolled`/`TurnStarted` publish path `SetMode` always
used for any other FreeRoam→TurnBased flip. Called inline at the mutation sites (`Move`,
`AddMonster`) rather than as a kicked/deferred check — a forgotten kick call
would silently mean combat never starts, a worse failure mode than a
redundant inline check. The mode gate at the top makes repeated calls
idempotent (no re-roll, no "already TURN_BASED" error) once combat has
started.

"Hostile" == "is a monster" for wave 1 — no faction model exists yet, so
player-player visibility never triggers this, and every monster is a valid
trigger for every player. `AddMonster` checks visibility against existing
players (a monster's visibility to anyone is inherently "newly formed" the
moment it's added — there's no "before" state for an entity that didn't
exist); `Move` checks after the mover's `View` has updated, so a player
walking around a corner into a goblin's hex triggers combat the moment the
wall no longer blocks LoS between them.

**Pre-hydration entry + the turn-seed catch-up.** Combat entry can now fire
on an **un-hydrated** encounter: the production creation flow is
`New()`+`AddPlayer`+`AddMonster` (rpg-api's `StartEncounter`), and `New`
never hydrates — only a `LoadFromData` round-trip does. `SetMode`'s
`seedActorTurn` finds no held character at that moment and correctly skips,
but `SetMode` and `EndTurn` were the only two seeding sites — so the first
active player's action economy would never be seeded, and every `TakeAction`
would fail `"not in combat"` (a latent gap that predates #757 for any host
calling `SetMode` on a fresh encounter — devseed's `--inject-combat` shape —
which combat entry turned into the default path; surfaced as a ~50%-flaky
`TestResolver_ReceivesHeldEntity` during #757's test run, the flake being
initiative order). The fix: `LoadFromData` ends with
`seedActiveActorIfUnseeded` (turn_economy.go) — if the mode is TURN_BASED
and the active actor's held character reports `!InCombat()` (economy nil,
i.e. never seeded; the economy persists through `ToData`/`LoadFromData`, so
a mid-turn reload sees `InCombat() == true` and is never re-seeded), it runs
the missed `seedActorTurn` and pushes a fresh `TurnStateChangedEvent`
(Invariant 12 — the flip-time push snapshotted an un-hydrated empty menu).
No `TurnStartedEvent` re-publish: the turn already started and was announced
at the flip; the catch-up completes its seeding, it doesn't restart it.

### Monster visibility events (rpg-toolkit#761, #764)

`checkCombatEntry` flips the encounter mode on a newly-formed player-monster
sightline, but flipping the mode alone doesn't tell a client which monster it
can now see — `EntityAppearedEvent`/`EntityDisappearedEvent` previously only
fired for the player-sees-player direction (`applyAndPublishMove`, via
`perception.ProjectMove` + `perception.ProjectVisibilityTransition`), so a
goblin sighted by the same move that started combat never appeared
client-side.

`Encounter.monsterVisibilityTransitions` (combat.go) closes this by reusing
that exact machinery for the player-sees-monster direction. Each stationary
monster is modeled as a synthetic, non-moving `perception.View` at the
monster's own position, carrying the moving player's own sight range — so
"can the player see monster M" and "can a stationary viewer at M's hex, with
the player's sight range, see the player" evaluate identically, the same
predicate `checkCombatEntry` already uses via `perception.CanSeeAt(player,
monster)`.

This substitution's validity is **bounded, not unconditional**:
`CanSeeAt`/`VisibleHexesAt`'s wall check treats the two compared positions
symmetrically only for distances below 22 hexes on the current grid.
`HexGrid.lerpCube` (`tools/spatial/hex_grid.go:528`) truncates its
interpolated cube coordinates with `int()` instead of rounding, so
`GetLineOfSight`'s interior-cell set for A→B and B→A starts to diverge at
distance 22 hexes (concrete counterexample: player `{0,0,0}`, monster
`{9,-22,13}`, wall `{6,-14,8}` — one direction is blocked, the reverse
isn't). Wave 1's sight ranges max out at 10, well inside the safe zone, so
this cannot manifest today — but a future sense with range ≥22 hexes (e.g.
120ft darkvision = 24 hexes) would cross the boundary. The `lerpCube`
truncation is a pre-existing gap tracked as a follow-up issue; it was not
fixed as part of #761.

Wired into `applyAndPublishMove`, immediately after the existing
player-sees-player transition loop — not into `checkCombatEntry` — because
`checkCombatEntry`'s `ModeFreeRoam` gate exists to make repeated
`Move`/`AddMonster` calls idempotent for the *entry* transition only.
Detecting monster visibility there would silently stop firing
appear/disappear events for the rest of the encounter the moment combat
started, missing the ongoing-combat case (a player losing sight of a monster
mid-fight by moving around a corner) this issue also needs covered.
`applyAndPublishMove` runs on every `Move` regardless of mode, so it's the
correct home for a general per-move visibility diff.

One difference from the player-sees-player case: a monster's published
`Position` (appeared) / last-known hex (disappeared) is always its own fixed
hex, never the transition hex `ProjectVisibilityTransition` computes — that
hex lives on the *player's* path (where the player crossed into or out of
range) and is meaningless for where to draw an entity that never moved.

Scope: player-move side only. `npc.go`'s simpler NPC-move visibility model
(tracked separately, #637) and populated `View.KnownEntities` remain future
work, unchanged by this issue.

**The spawn-side gap (#764).** #761/#762 only wired detection into
`applyAndPublishMove` (the player-Move path), so a monster added via
`AddMonster` directly into an already-visible position started combat
(`checkCombatEntry` fired) but never told the client which monster it was —
found by the review gate on PR #762. `AddMonster` now also publishes
`EntityAppearedEvent` for every player who can already see the newly-added
monster (`Encounter.playersWhoCanSee`, combat.go), published *before*
`checkCombatEntry` so the appearance precedes the `ModeChangedEvent` it
causes, same ordering contract as the Move path. Unlike
`monsterVisibilityTransitions`, this needs no `ProjectVisibilityTransition`
machinery: a freshly-added monster has no "before" state to diff against —
its visibility to any existing player is inherently newly formed the moment
it exists (the same reasoning `checkCombatEntry`'s own doc comment already
gives) — so a plain per-player `CanSeeAt` scan is sufficient. Deliberately
NOT gated on encounter mode: a reinforcement or door-triggered spawn added
mid-combat must still appear even though `checkCombatEntry`'s
`ModeFreeRoam` gate makes the mode-change half of the check a no-op at that
point. When more than one player can already see the new monster (only
possible via `AddMonster` — a single player `Move` can only ever change
that one player's own visibility), all of them are grouped into one
`EntityAppearedEvent`'s `PerPlayer` set, since the monster's `Position` is
the same fixed hex for every viewer.

Considered folding `npc.go`'s NPC-move-side gap (#637) into the same change
since the issue named it as "same family" — decided against it: `AddMonster`
needs no transition machinery at all (see above), while the NPC-move case
is a genuine moving-entity problem needing the *player-sees-player* shape of
`ProjectVisibilityTransition` (stationary viewers, a real moving entity) —
different mechanics, already tracked separately, and folding it in here
would have widened this fix past a single reviewable change.

### Initiative roster event (rpg-toolkit#765)

`SetMode`'s FreeRoam→TurnBased flip always rolled `data.Initiative`
(`rollInitiative`) but published no event carrying it — only
`ModeChangedEvent` (the transition itself) and `TurnStartedEvent` (the first
actor's turn) hit the wire. Consumers that need the roster had to read it
back from persisted state, which races the orchestrator's Save (the toolkit
publishes synchronously inside the verb call, before the caller persists) —
rpg-api's stream handler carried a bounded 15×10ms retry around exactly this
(rpg-api#647), plus a synthesized envelope that reused `ModeChangedEvent`'s
sequence number since the handler has no way to mint a real broker sequence.

`SetMode` now publishes `events.InitiativeRolledEvent{Order
[]core.EntityID}` between `ModeChanged` and `TurnStarted`. Design choice —
**dedicated event, not fields on `ModeChangedEvent`**:

- The wire proto already models this as its own message
  (`InitiativeRolled{order}`, `EncounterEvent` oneof field 41, added ahead of
  this toolkit seam and left unpopulated until now). A dedicated toolkit
  event maps onto it 1:1, so rpg-api's translator drops the special-cased
  `translateForStream` branch entirely (repo read, retry, shared-sequence
  hack, and all) and treats `InitiativeRolledEvent` like every other event —
  a plain `TranslateEvent` case. Growing `ModeChangedEvent` with roster
  fields instead would still leave that branch in place (still splitting one
  broker event into two wire envelopes by hand), just without the repo read.
- `ModeChangedEvent` is generic — it fires for every mode pair, including
  TurnBased→FreeRoam, where a roster field would have to be empty/absent.
  Every other roster-adjacent fact in this taxonomy (`TurnStarted`,
  `TurnEnded`, `EntityDied`, `EntityRemoved`) is already its own dedicated
  event type; bundling roster data onto the mode-transition event breaks
  that precedent for a shape that only makes sense on one specific
  transition.

**Ordering contract**: `ModeChangedEvent` → `InitiativeRolledEvent` →
`TurnStartedEvent`. `ModeChanged` is the cause; both the roster and the
first turn are effects of it, but the roster (STATE — "here is the full
turn order") is placed before the turn announcement (an ACTION — "it's this
actor's turn") since a client building a combat-order UI wants the full
roster before being told who's first. Published only on the FreeRoam→
TurnBased direction, and only once per transition — not re-sent on every
later per-turn `TurnStartedEvent` from `EndTurn`, since the roster doesn't
change turn to turn. Pinned by
`TestSetMode_InitiativeRolled_SequencedBetweenModeChangedAndTurnStarted`,
`TestEndTurn_DoesNotRepublishInitiativeRolled`, and
`TestSetMode_FreeRoamTransition_NoInitiativeRolled`
(`initiative_rolled_test.go`).

The published `Order` is a defensive copy of `e.data.Initiative`, not the
same backing slice — mirrors `applyAndPublishMove`'s
`append([]core.Hex(nil), traveledPath...)` pattern, since `AddMonster`'s
mid-combat reinforcement path (`e.data.Initiative = append(e.data.Initiative,
input.ID)`, #757) can grow the roster after this event already published.

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
  the dnd5e rulebook's set, senses, per-viewer wall reveal (wave 1 ships
  whole-room wall visibility — see "Walled rooms" below), faction model
  (combat-entry's "hostile" == "is a monster" is a wave-1 stand-in),
  Redis transport, gRPC handler. Entity-visibility accumulation is
  reserved in the type shapes (`HexRevealedSlice.Entities`,
  `View.KnownEntities`) but not emitted yet — future slice.

Catch-up policy: snapshot-only on reconnect (load `Snapshot`, attach live stream). Event-replay catch-up is a future slice that adds an `EventLog` interface alongside `Transport`. Sequence numbers on events are already in place; the addition is non-breaking.

## TakeAction unifies the verb path with the character economy/menu (rpg-toolkit#697, ADR-0032)

`TakeActionPhased` no longer hard-gates on `ref.ID == "attack"`. The attack ref
keeps its two-phase resolver path (above); **every other ref delegates to the
held character's own rules engine** via `takeCharacterAction` —
`character.ActivateAbility` for abilities/features, `character.ExecuteAction` for
granted-capacity actions. No ref is enumerated in the encounter; the character's
menu is the membership test. The action runs on the `*character.Character` the
`LoadFromData` cascade already holds on `e.bus` (ADR-0030) — never a re-load.

The non-attack catalog seeded on every character — Dodge, Dash, Disengage, Help,
Hide — all flow through this one verb. Help/Hide constructors landed in the dnd5e
half (rpg-toolkit#702, v0.61.0); the encounter module pins v0.61.0.

Turn-start economy seeding moved into the engine: `Encounter` calls
`character.StartTurn` on the held character at each turn boundary (the
`SetMode→TURN_BASED` first actor and `EndTurn`'s next actor), so rpg-api no longer
injects `ActionEconomyData{1,1,1}` (North-Star Invariant 2).

`Encounter.ActorTurnState(actorID)` exposes the held character's two-level menu
(`AvailableAbilities` + `AvailableActions`, each with an `EconomySlot` and a
`TargetKind`) and economy as toolkit domain types — the read surface rpg-api
projects to the wire `TurnState` field-for-field (Invariant 11). NPC and
flat-stat seats return an empty `ActorTurnState`.

**Turn state is push-refreshed, never silently stale (Invariant 12, ADR-0033).**
Whenever an actor's turn state mutates — turn start (seeding) and every action
taken (economy deducted) — the encounter publishes a `TurnStateChangedEvent`
through the broker carrying a full snapshot (economy + menu) of
`ActorTurnState`, flattened to rulebook-agnostic primitives
(`events.TurnStateSnapshot` / `MenuEntry`) so the spine stays rulebook-free.
Audience is the actor's own controlling player (their private "what can I do
now" view). The post-action push shares the causing action's correlation id
(Invariant 8); the turn-start push carries none (not caused by an action).
rpg-api projects this onto the proto `TurnStateChanged` (envelope field 45).

The character menu reports rules truth (a level-1 character *can* move, so
`Move.CanUse == true`), but `ActorTurnState` composes the **effective**
takeability the wire `available` flag means: refs this build defers (the
`deferredActionRefs` set — `move` is its one Beat-1 member, movement lands in
Beat 2) are projected `available=false` with an honest reason, and the verb
rejects them with `ErrActionDeferred` so menu and verb never disagree (D17,
ADR-0032). Beat 2 removes the entry in one line.

Every action now emits a first-class `ActionResolvedEvent` (Invariant 9): the
non-attack path publishes the umbrella event with the real ref + economy
consumed; the attack path threads the actor's real submitted ref + economy
instead of the prior placeholder constant.

Known gaps left as follow-ups (see ADR-0032): Dodge's `DodgingCondition` is not
yet applied on `DodgeActivated`; the two Monk-bonus-strike implementations are
not collapsed; the `CompleteTakeAction` resume path reports the attack default
ref because `PhasedAttackContext` does not carry the submitted ref.
