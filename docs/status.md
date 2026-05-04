---
name: rpg-toolkit status
description: Where we are with rpg-toolkit — active work, paused, known rough edges, per-subsystem confidence
updated: 2026-05-02
confidence: medium — seeded from full repo read, test run, go.mod inspection, and PR history; needs Kirk's correction pass on any items that have already moved
---

# rpg-toolkit: Where We Are

This is a living doc. Edit it in the same PR that invalidates a line. Don't let it rot.

## Active work

No open PRs as of 2026-05-02. The last merge was PR #609 (fix/456-unarmored-defense-ac-chain,
2026-04-05). Main is quiet.

A large number of stale remote branches remain (40+) from earlier exploratory work.
They are not merged and likely not resumable as-is. See "Paused / on hold" below.

## Recently landed (last 30 days, highlights)

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

- **`items/validation` tests do not compile.** The mock in
  `items/validation/basic_validator_test.go` implements `GetType() string` but
  `core.Entity` requires `GetType() EntityType`. The `items` module pins
  `core v0.1.0` and has a local `replace` directive pointing to `../../core`.
  The current `core` at that path defines `EntityType`, so the mocks in the test
  are simply stale. This is a broken test, not broken production code, but it
  means `go test ./...` from the items module fails at compile. No open PR.

- **`mechanics/conditions/go.mod` and `mechanics/spells/go.mod` need `go mod tidy`.**
  Both have local `replace` directives and pinned versions that have drifted.
  Running tests works today but CI will flag a diff in go.mod. Confirmed: running
  `go test ./...` in conditions returns `go: updates to go.mod needed` before
  printing test results.

- **`items`, `mechanics/proficiency`, `mechanics/spells`, `mechanics/conditions`
  all carry local `replace` directives.** This violates the stated workspace rule
  ("NEVER add replace directives — breaks CI/CD"). The workspace CLAUDE.md allows
  them during dev but they must be stripped before commit. These are currently
  committed to main.

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
| rulebooks/dnd5e/integration | High — Barbarian, Fighter, Monk, Rogue encounter tests all pass |
| rulebooks/dnd5e/dungeon | Medium — tests present; planned to move out of rulebook |
| items | Low — base module has no tests; validation tests do not compile |
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

### Items module repair

`items/validation` tests need mock types updated to use `core.EntityType`. Likely
a 30-minute fix but needs an issue.

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
