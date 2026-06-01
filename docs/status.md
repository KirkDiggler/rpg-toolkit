---
name: rpg-toolkit status
description: Where we are with rpg-toolkit — active work, paused, known rough edges, per-subsystem confidence
updated: 2026-06-01
confidence: medium — seeded from full repo read, test run, go.mod inspection, and PR history; #689 + Wave 2.11d updates verified against shipped code
---

# rpg-toolkit: Where We Are

This is a living doc. Edit it in the same PR that invalidates a line. Don't let it rot.

## Active work

**#697 (TakeAction wave, event-faithfulness PR) — encounter event spine:
causation + game-event time + resolved-action event (2026-06-01).**
Added `OccurredAt()` / `CorrelationID()` / `Stamp()` to the `EncounterEvent`
interface (single-sourced on an embedded `eventMeta`), made `Broker.Publish` the
single game-event-time stamp authority via an injected `core.Clock`
(`NewBrokerWithClock`; default `SystemClock`, tests use `FixedClock`), and added
the first-class `ActionResolvedEvent` (`action_ref` + `economy_consumed`).
Attack resolution now publishes a correlated `ActionResolved → AttackResolved →
DamageDealt` group sharing one correlation id. See ADR-0031. This is the FIRST
of two separable #697 PRs; the menu/economy unification (reconcile the encounter
verb path with the character action menu, non-attack actions, validate+deduct)
is the next chunk. Proto mirror (new `ActionResolved` oneof variant +
`occurred_at`/`correlation_id` on the envelope) is the dependency-ordered
follow-on before rpg-api un-suppresses.

**#689 — encounter owns combatant hydration via the LoadFromData cascade
(2026-05-30, cross-repo unit with rpg-api#582; NOT yet merged).**
`Encounter.LoadFromData(ctx, ...)` now cascades into each combatant's own
`LoadFromData` (players from new `PlayerData.DataJSON`, monsters from
`MonsterData.DataJSON`), holds the runtime entities as `combat.Combatant`, and
applies their conditions to the bus exactly once — the source-level cure for the
#684 "modifier ID already exists" double-subscribe class. Resolvers receive the
held entity (`AttackInput.Attacker/Defender`, `MovementStepInput.Mover`) and
never re-load; `EndTurn(ctx, ...)` emits the dnd5e turn-boundary on the bus so
held conditions reset with no re-load; `ToData()` cascades held state back.
Breaking: `ctx` on `LoadFromData`/`EndTurn`/`New`. Bumped `rulebooks/dnd5e` to
v0.59.0. See ADR-0030 + journey 050. The host (rpg-api#582) wires against a local
replace; toolkit PR ships a real tag at the end of the unit.

**Wave 2.11d toolkit half landed (PR #656, 2026-05-14)** — opt-in player
reactions through the v2 stack. Toolkit ships the SDK surface; rpg-api
integration and web halves are next.

What landed on the toolkit side:

- `combat.AttackContext` is now pure data (eventBus/roller removed;
  `AbilityMod`/`AbilityUsed`/`IsOffHandAttack` exported). JSON
  round-trippable so the host can persist it across the player-reaction
  RPC gap.
- `combat.ApplyAttackOutcomeInput` carries `EventBus` + `Roller`
  directly (symmetric with `ResolveAttackHitInput`). `EventBus` required;
  `Roller` defaults.
- `combat.PostAttackRollChain` typed chained-topic published in
  `ResolveAttackHit` after `wouldHit` computation — Shield's predicate
  subscribes here.
- `encounter.PhasedCombatResolver` interface (optional extension to
  `CombatResolver`); `Encounter.TakeActionPhased` + `CompleteTakeAction`
  are the canonical orchestration entry points.
- `Encounter.Data.PendingReactionPrompts` is the persistence shape between
  phase 1 and phase 2; `encounter/events.InputRequiredDeliveredEvent` is
  the single-viewer audience SDK event the translator reads to build the
  proto payload.
- NPC pause-on-reaction via `errNPCPausedForReaction` sentinel +
  `IsNPCPausedForReaction(err)` helper for orchestrators.
- Two new conditions in `rulebooks/dnd5e/conditions/`: `OpportunityAttack`
  (MovementChain subscriber) and `Shield` (PostAttackRollChain
  subscriber). JSON round-trip pattern is now exercised by 4 conditions.
- Known follow-up: issue
  [#657](https://github.com/KirkDiggler/rpg-toolkit/issues/657) tracks the
  HOST CONTRACT smell on `PendingReactionPrompt.AttackContextJSON` (SDK
  trusts host to populate; resolver-supplied serializer is the proper fix).

**Encounter SDK walking skeleton (#622)** — Phase 2 Slice 1 of v1alpha2,
landed earlier. New top-level `encounter/` module with subpackages `core`
(IDs + spatial primitives), `events` (sealed taxonomy + AudienceSet),
`perception` (projection functions). Sealed `EncounterEvent` interface
(AWS v2 SDK marker pattern). Process-scoped `Broker` over a pluggable
`Transport` (InMemoryTransport; Redis/Kafka are future). Transient
`Encounter` aggregate with `Move` and `OpenDoor` verbs,
`ToData`/`LoadFromData` persistence, `SnapshotFor` for stream snapshots.
Stub Manhattan-radius LoS in `encounter/perception/`; real LoS is a
future slice.

Earlier active state: no open PRs as of 2026-05-02; last merge was PR #609.
A large number of stale remote branches remain (40+) from earlier
exploratory work. They are not merged and likely not resumable as-is.
See "Paused / on hold" below.

## Recently landed (last 30 days, highlights)

- **Move-iteration OA damage application** — PR for issue #675 (2026-05-24) — Wave 2.11e
  SDK seam: `iterateMovementStepsForEntity` now captures `DamageReceivedEvent` on the
  encounter bus per step (alongside the existing `ReactionTriggerTopic` buffer) and
  dispatches HP delta + encounter-side `DamageDealtEvent` via new `applyMoveDamage`
  helper. Without this, Move-path OAs fired and rolled but never moved target HP — the
  goal sentence "OA-class reactions work end-to-end" silently failed at the encounter
  boundary. Mirror of `applyCapturedDamage` pattern with polymorphic source lookup.
  Director-review fix-up (#677) scoped NPCAct's outer damage subscriber to the
  attack-resolution window only, eliminating a double-apply path where movement-OA
  damage flowed through both the inner per-step subscriber AND the outer subscriber.
- **Monk Unarmored Defense WIS AC chain** — PR #609 (2026-04-05) — adds WIS modifier
  to AC when Monk is unarmored and has no shield; test covers the full chain.
- **Martial Arts DEX label fix** — PR #607 (2026-04-05) — `SourceRef.Label` was
  "STR" when Martial Arts overrides to DEX. Cosmetic but needed for correct breakdowns.
- **Unarmed strike damage / AbilityUsed propagation** — PR #604 (2026-03-29) —
  registers unarmed strike as a weapon, threads `AbilityUsed` through damage chain.
  Copilot review feedback addressed in follow-up commit.
- **Condition remove cleanup** — PR #603 (2026-03-22) — `Remove()` now collects all
  unsubscribe errors instead of returning on first failure.
- **UnconsciousCondition with death save automation** — PR #601 (2026-03-22).
- **EquipmentDetail types** — PR #600 (2026-03-22) — resolves equipment details,
  implements Equipment interface on Item.
- **Unified action economy types and persistence** — PR #597 (2026-03-22) —
  `ActionEconomyData` with `TurnNumber` tracking. Deep-copy on access; idempotent
  `AddCombatAbility`.

## Paused / on hold

- **Stale feature branches** — `origin/feat/505-attack-resolution`,
  `origin/feat/505-movement-integration-tests`, `origin/feat/546-character-speed-extra-attacks`,
  `origin/feat/546-turn-end-cleanup`, and a dozen others. No corresponding open PRs.
  Need triage: close if superseded, resume if still needed.
- **Behavior system** (`behavior/doc.go` only) — ADR-0016 documents the intent
  but the directory is empty. Deferred indefinitely.
- **Experiences architecture** (`docs/adr/0017-experiences-architecture.md`) —
  ADR exists, no implementation.
- **Content provider interface** (ADR-0018) — Same state.
- **`spawn/doc.go`** (root-level, not `tools/spawn`) — Stub only; superseded by
  `tools/spawn` which is complete.

## Known rough edges

### Module hygiene — active build failures

- **`mechanics/conditions/go.mod` and `mechanics/spells/go.mod` carry committed
  local `replace` directives** that mask source drift against the events
  module. Their main-branch source uses the **old events API**
  (`events.Event`, `events.HandlerFunc`, `event.Context().GetString()`,
  `event.Context().AddModifier()`) — a shape that no published events version
  exposes today. The replace directives point `events => ../../events` so the
  build can find these symbols somewhere; without that pointer the source
  doesn't compile. The 4-class playtest doesn't exercise either module so
  this is deferred — tracked as **issue #617**. Issue #613 (the directive
  cleanup) had its items + proficiency portion resolved 2026-05-04; the
  conditions/spells portion rolls into #617.

- **events API rewrite, not version bump.** The events module has been
  rewritten on main from a typed-event API (`events.Event`, `HandlerFunc`,
  `Context().GetString()` / `.Set()` / `.AddModifier()`) to a typed-topic API
  (`TypedTopic[T]`, `ChainedTopic[T]`, `BusEffect`, `StagedChain`). Two
  worlds today:
  - **New API (current main events surface):** rulebooks/dnd5e (+ subpackages),
    tools/spatial, tools/environments, tools/spawn, tools/selectables. These
    pin events v0.6.x.
  - **Old API:** mechanics/effects (matches published v0.2.1),
    mechanics/conditions, mechanics/spells, mechanics/features, game,
    mechanics/proficiency. These pin events v0.1.x in their go.mod;
    conditions/spells additionally use APIs not present in v0.1.0, which is
    why their replace directives point at local source. Proficiency builds
    cleanly against v0.1.0 because it only consumes effects (which itself
    works against v0.1.0).
  - **No events dependency at all:** items.

  Closing #617 means rewriting effects/conditions/spells/features against
  the new typed-topic shape, not version-bumping the events line. That is
  a real refactor, not a hygiene task.

### Spatial

- **`PathFinder` interface is hex-only.** `SimplePathFinder.FindPath` takes
  `CubeCoordinate` arguments. Square and gridless rooms have no in-room pathfinder
  — callers must use the multi-room `Orchestrator.FindPath` (room-level only) or
  do their own A* for square grid intra-room movement. This is undocumented as a
  gap.

- **`LayoutOrchestrator` and `TransitionSystem` interfaces are defined but not
  implemented.** Documented in `tools/spatial/CLAUDE.md` as "future work" but the
  unimplemented interfaces sit next to implemented ones without a `// not implemented`
  marker. Easy to be confused about what is callable.

- **No pathfinder tests cover cycles or very large grids.** `pathfinder_test.go`
  covers direct path, L-shaped wall, surrounded (no path), same position, and
  blocked goal — that is five cases. There are no tests for large grids, performance
  bounds, or edge cases around the priority queue (equal-cost nodes).

### dnd5e rulebook

- **`dungeon` subpackage lives in `rulebooks/dnd5e/dungeon/`.** Per the project plan
  (rpg-project/CLAUDE.md and this team's architecture discussions), dungeon logic
  is slated to move to a toolkit-level package. The current location creates an
  implicit dependency path: dungeon → environments → spatial, all inside the
  rulebook. See "Upcoming work" below.

- **Several `dnd5e` subpackages have zero test files:**
  `abilities`, `ammunition`, `armor`, `backgrounds`, `damage`, `effects`,
  `fightingstyles`, `items`, `languages`, `packs`, `proficiencies`, `race`,
  `races`, `refs/abilities` (only `refs_test.go` covers the whole refs package).
  These are mostly data/constant packages, but `backgrounds` and `races` include
  grant logic (`grants.go`) with no tests.

- **`character/choices` has testdata from a DnD 5e API** (`testdata/api/classes/`,
  `testdata/api/races/`). The provenance and freshness of this data is not
  documented. If the upstream API changes, tests silently test stale data.

### Events

- **`events/bus.go` and `events/bus_effect.go`** — the dual-bus pattern (plain bus +
  effect bus) is unexplained in any ADR. ADR-0024 covers typed topics but not the
  two-bus split. Easy for new contributors to wire the wrong one.

### Mechanics

- **`mechanics/features`** has no test files at all (only `mock/`). The feature
  loader and `SimpleFeature` are untested directly.

- **`mechanics/spells`** has test files but the go.mod has a `go mod tidy` needed
  warning, and the replace directives mean CI state is unclear.

## Per-subsystem confidence

See quality.md for grade and rationale.

| Subsystem | Confidence |
|---|---|
| core | High — stable foundation, clean interfaces, good tests |
| events | Medium-high — typed topics work well; dual-bus split undocumented |
| dice | High — well-tested including pool, lazy, modifier, notation |
| encounter | High — #689 made LoadFromData own combatant hydration (held entities, #684 double-subscribe cured at source); discrete-phase orchestration + reaction prompts from Wave 2.11d; HOST CONTRACT smell on AttackContextJSON tracked in #657; ActivateFeature self-load (defer Cleanup) folding into the held entity is a tracked follow-up |
| mechanics/conditions | Medium — good coverage for dnd5e conditions; base module has go.mod drift |
| mechanics/resources | Medium-high — passes tests, no known gaps |
| mechanics/effects | Medium — no suite-pattern tests; functional |
| mechanics/features | Low-medium — no tests in base module |
| mechanics/spells | Medium — go.mod drift; tests present but CI unclear |
| mechanics/proficiency | Medium — replace directive in go.mod |
| tools/spatial | High — comprehensive hex/square/gridless + orchestrator; pathfinder gap in square grid |
| tools/environments | Medium-high — persistence and pathfinding covered; thin on edge cases |
| tools/selectables | Medium-high — passes, good pattern |
| tools/spawn | Medium-high — 4-phase implementation complete; environment integration tested |
| rulebooks/dnd5e (core) | High — character, combat, conditions, features all tested |
| rulebooks/dnd5e/combat | High — Wave 2.11d shipped AttackContext-as-pure-data refactor + PostAttackRollChain seam; phase 1/phase 2 input symmetry is clean |
| rulebooks/dnd5e/conditions | High — Wave 2.11d added OpportunityAttack + Shield; JSON round-trip pattern now exercised by 4 conditions (sneak attack, raging, OA, Shield) |
| rulebooks/dnd5e/integration | High — Barbarian, Fighter, Monk, Rogue encounter tests all pass |
| rulebooks/dnd5e/dungeon | Medium — tests present; planned to move out of rulebook |
| items | Low — base module has no tests; validation tests pass after issue #612 fix |
| rpgerr | High — scenario tests and accumulation tests cover the patterns |
| game | Medium-high — context pattern tested |
| behavior | Low — empty implementation, ADR only |

## Upcoming work

### Dungeon component inbound move (expected)

The `rulebooks/dnd5e/dungeon/` package is slated to move to a toolkit-level
location (likely `tools/dungeon/` or a new top-level `dungeon/` module). This
will:
- Break the dependency from rulebook down to environments/spatial.
- Allow rpg-api to use dungeon logic without importing the full dnd5e rulebook.
- Require updating all callers in rpg-api.

No issue or branch exists yet. Treat this as pre-planned but unscheduled.

### go.mod replace directive cleanup

`mechanics/conditions`, `mechanics/spells`, `mechanics/proficiency`, and `items`
all have replace directives committed to main. Each needs a cleanup PR to pin
real published versions and remove the directives.

### Stale branch triage

40+ remote branches with no open PRs. A triage pass to close or label them would
reduce noise.

## Related references

- [rpg-project CLAUDE.md](../../rpg-project/CLAUDE.md) — cross-repo boundary rule
- [rpg-project milestones/4class-dungeon/](../../rpg-project/milestones/4class-dungeon/) — current milestone
- [Project board #10](https://github.com/users/KirkDiggler/projects/10)
- [docs/adr/](adr/) — 29 ADRs covering major design decisions
- [docs/journey/](journey/) — 48 journey docs, exploration history
