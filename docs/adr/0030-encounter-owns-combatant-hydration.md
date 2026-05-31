# ADR-0030: Encounter Owns Combatant Hydration via the LoadFromData Cascade

Date: 2026-05-30

## Status

Accepted

## Context

The `encounter` SDK held no hydrated rulebook entities. `Encounter.LoadFromData`
created a fresh dnd5e event bus but hydrated nothing; the combat/movement
resolvers (`CombatResolver`, `PhasedCombatResolver`, `MovementResolver`) were
handed only entity IDs + stat snapshots and were documented as responsible for
re-loading the rich rulebook entity "from whatever store the orchestrator wires
in."

Because the host (rpg-api) had to re-load each character/monster per attack and
per turn-boundary to re-`Apply` its conditions onto the **same** encounter bus,
the same condition subscribed to the bus more than once. Two
`SneakAttackCondition` instances, for example, would each try to add the
`"sneak_attack"` modifier to the damage chain, and the second chain `Execute`
returned `events.ErrDuplicateID` ("modifier ID already exists") — the #684
double-subscribe class. It was patched in the host with `defer Cleanup`, not
cured.

`character.LoadFromData` (`rulebooks/dnd5e/character/data.go`) already cascades
into `conditions.LoadJSON` and calls `condition.Apply(ctx, bus)` per condition —
that `Apply` IS the single subscribe point. The bug was calling `LoadFromData`
on the same bus twice, not the cascade itself.

## Decision

**`Encounter.LoadFromData` owns combatant hydration via the existing
`ToData`/`LoadFromData` round-trip, which composes.** Hydration is not a new
subsystem (no injected "hydrator" interface) — it is the engine's standard
serialized↔runtime pattern applied one level up:

- `LoadFromData(ctx, data, broker, opts...)` cascades into each combatant's own
  `LoadFromData`: players from the new `PlayerData.DataJSON` via
  `character.LoadFromData`; monsters from `MonsterData.DataJSON` via
  `monster.LoadFromData` + `monsteractions.LoadMonsterActions` +
  `monstertraits.LoadMonsterConditions`. Default reaction conditions (OA/Shield),
  driven by the SDK-owned `ReactionReadiness` map, are applied in the same step.
- The runtime entities are **held** on the `Encounter` as `combat.Combatant`
  (`e.combatants`), reconstructed (not serialized) each load — exactly like
  `e.bus`. This single cascade is the **only** place conditions `Apply` to the
  bus: one load, one subscribe (the #684 cure).
- The resolver seam receives the **held** entity:
  `AttackInput.Attacker/Defender` and `MovementStepInput.Mover` are
  `combat.Combatant`. Resolvers use them and MUST NOT re-load. They are nil when
  a seat carried no rehydratable data, and the resolver falls back to its
  stat-snapshot stand-in.
- `EndTurn(ctx, actorID)` emits the rulebook turn-boundary
  (`dnd5eEvents.TurnEndTopic`) directly on `e.bus` for the ending actor, so held
  conditions reset per-turn state (`SneakAttack.UsedThisTurn`) in place with no
  re-load. (No pluggable signaler — the package already imports `dnd5eEvents`.)
- `ToData()` mirrors the cascade: it re-serializes each held entity's `ToData()`
  back into the owning `PlayerData/MonsterData.DataJSON` so the next load sees
  current state, replacing the host's scattered `saveAttackerConditionState` +
  `publishTurnEndAndPersistReset` write-back.

`NPCAct` reuses the cascade-held monster when present (only loading when there is
no held instance — the `New`-without-`LoadFromData` path), so it does not
re-subscribe the monster's conditions to the bus.

### Boundary stance

The `encounter` SDK is dnd5e-coupled by precedent: `npc.go` already called
`monster.LoadFromData`, `activate_feature.go` already called
`character.LoadFromData`, and `encounter.go` already imported `dnd5eEvents`. This
cascade makes that coupling **coherent and single-sourced** rather than scattered
across the host. A "fully agnostic engine" (pluggable rulebook loaders behind an
interface) remains separately tracked; it is not a blocker for this work. This is
the ratified call (see `rpg-project/ideas/encounter/v1alpha2/plans/10-689-encounter-hydration-cascade.md`).

### `ToData` is not dirty-gated (code-over-doc correction)

The ratified plan said the `ToData` write-back would be gated on the rulebook
entities' existing `IsDirty()`. On reading the code, `Character.dirty` is set
**only on HP change** (`character.go`), NOT on condition-state mutations like
`SneakAttack.UsedThisTurn`. Gating on `IsDirty()` would silently drop the
per-turn condition state that #689 must round-trip for cross-RPC once-per-turn
enforcement. So the write-back is **unconditional** for held entities (one JSON
marshal per combatant per `ToData`); `character.ToData()` already re-serializes
every held condition via `condition.ToJSON()`, capturing the current state.
`MarkClean()` is still called for consumers that track HP dirtiness. Code wins
over the plan here.

## Consequences

### Positive
- The #684 double-subscribe class is cured at the source: one load, one
  subscribe, proven by `encounter/hydration_test.go` (a `SneakAttack` condition
  fires N times across attacks in one encounter on the real broker with no
  `ErrDuplicateID`; the test fails when double-hydration is simulated).
- The host (rpg-api) shrinks: `resolveEntity`, `loadCharacterWithBus`, the
  `pendingPhasedAttack` double-load cache, `saveAttackerConditionState`,
  `publishTurnEndAndPersistReset`, and `reaction_conditions.go` (the
  depguard-excluded boundary violation) all delete. The resolver becomes pure
  translation.
- Cross-RPC once-per-turn state (`UsedThisTurn`) round-trips through the
  `ToData` cascade without a separate host write-back.

### Negative
- Breaking change to published interfaces: `LoadFromData` and `EndTurn` (and
  `New`) gain `ctx`; `AttackInput`/`MovementStepInput` gain held-entity fields;
  `PlayerData` gains `DataJSON`. Consumers update in the same cross-repo unit.
- `ToData` now does a JSON marshal per held combatant per call (modest cost,
  correctness-critical given the dirty-flag gap above).

### Neutral
- The `encounter` module bumped its `rulebooks/dnd5e` dependency to v0.59.0
  (the first tag carrying the OA/Shield condition constructors + loader routing
  and `monstertraits.LoadMonsterConditions`).
- `ActivateFeature` still loads its own character with `defer Cleanup`
  (`activate_feature.go`); folding it onto the held combatant is a tracked
  follow-up (same #684 class, off the combat-resolution critical path).
- The cure rests on the turn-based, stateless-per-RPC model (bus reconstructed
  fresh each load). A future in-memory/real-time server would shift the
  invariant from "fresh per request" to "subscribe-once for the in-memory
  lifetime" — a separately-tracked evolution.

Confirmed by `encounter/hydration_test.go` (subscribe-exactly-once,
UsedThisTurn persist+reset, resolver-receives-held-entity, no-data fallback,
NPCAct-uses-held-monster) and the full `encounter` suite green under `-race`.
