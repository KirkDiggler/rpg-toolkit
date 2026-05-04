---
name: rpg-toolkit quality scorecard
description: Per-module grade with rationale — graded from code read, test run, and go.mod inspection 2026-05-02
updated: 2026-05-04
confidence: medium — first-pass grades from read-through and live test run; no coverage tooling run; should be reviewed by Kirk
---

# Quality scorecard

Every module graded A–D. Grades reflect: API clarity, test coverage, known gaps,
go.mod hygiene, architectural boundary compliance.

This is a first draft. Update grades in the same PR that changes the underlying
code.

---

## Infrastructure / Core

### core — A-

Clean Entity/EntityType interfaces, Ref/TypedRef for routing, well-typed error
hierarchy. Tests cover all exported types including `equipment_error` edge cases.
`generate.go` and `mock/mock_entity.go` follow repo conventions. Minor drag: the
`core/chain`, `core/combat`, `core/damage`, `core/effect`, `core/features`,
`core/resources`, `core/spells` sub-packages are all types-only with no test files.
None have executable logic so the risk is low, but changes to these types will not
be caught by CI tests — only by callers failing.

### rpgerr — A-

Structured error accumulation with RPG-domain context tagging. Scenario tests cover
the accumulation pattern end-to-end. Example accumulation test is illustrative.
Nothing obviously missing. Would benefit from a doc on when to use `rpgerr` vs
plain `fmt.Errorf`.

### game — B+

The `game.Context` pattern is clean and tested. Solves the "pass game data without
polluting function signatures" problem. Dependency on events and core is appropriate.
Pinned to old `events v0.1.1` and `core v0.1.0` — no replace directives, but the
version spread across modules makes it hard to know what "current game" means.

### events — B+

Typed topics via generics (`TypedTopic[T]`) are the right design for an event bus in
a strongly-typed language. `ChainedTopic` and `BusEffect` cover the modifier pipeline
use case. The dual-bus pattern (`EventBus` vs `BusEffect`) has no ADR explaining
when to use which. New contributors will default to the wrong one. All tests pass.
Example tests (`example_journey_test.go`, `example_magic_test.go`) are table-driven
but not in suite pattern — acceptable for examples.

---

## Dice

### dice — A-

Comprehensive: `Roller`, `Pool`, `LazyRoll`, `Modifier`, `Notation`, `Result` all
implemented and tested independently. Tests are not in suite pattern (flat
`TestXxx` functions) but coverage looks solid. `roller_new.go` alongside `roller.go`
is a naming smell — suggest renaming or collapsing. Mock provided. One gap:
`LazyRoll` behavior under extreme inputs (e.g., `d0`, negative count) is not tested.

---

## Mechanics

### mechanics/effects — B

Tracker, core, behaviors, composed condition — all pass tests. Infrastructure for
the condition/proficiency pipeline. Tests are flat (not suite pattern) but cover
meaningful behavior. The `mock/` subpackage exists but has no tests of the mock
itself. Grade held back from B+ because test style is inconsistent with the rest of
the repo and there is no explicit documentation of when to use `effects` vs
`conditions` directly.

### mechanics/conditions — B

The base module (`manager`, `simple`, `enhanced`, `builder`) is functional.
go.mod still carries 4 replace directives because the source has drifted past
published versions of the events API (issue #617). Cleaning up the directives
requires migrating the module to events v0.6.x; deferred until the playtest
exercises conditions in their newer form. `simple_test.go` and
`enhanced_test.go` are flat style (not suite). Actual condition behavior is
well-exercised at the `rulebooks/dnd5e` level which uses this module heavily.

### mechanics/resources — B+

Pool and counter pass. Clean resource management (spell slots, ki, rage uses).
No go.mod issues. Tests cover the main happy paths; edge cases (refill to zero,
consume past limit) could be more explicit.

### mechanics/effects (composed) — B

See mechanics/effects above — same module.

### mechanics/features — C

The `features/loader.go`, `features/feature.go`, and `features/simple_feature.go`
have **zero test files** in the base module. Only a `mock/` subpackage exists.
The feature loader is tested indirectly via `rulebooks/dnd5e/features`, but direct
unit tests for the base infrastructure are absent. For a module that other layers
depend on, this is a real gap. Grade would move to B with tests that exercise the
loader routing and error paths.

### mechanics/proficiency — B

`simple.go` has tests. go.mod is clean — replace directive removed (issue #613
resolved 2026-05-04). `proficiency.go` interface is clean but `doc.go` is the
only documentation of package-level intent. No examples.

### mechanics/spells — B-

Spell slots, concentration, spell list — all have tests and pass. The go.mod
still carries 6 replace directives (most of any module). Same root cause as
conditions: source has drifted past published events v0.1.x. Migration
deferred to issue #617 (playtest doesn't exercise spells yet). Tests are flat
(not suite) for most files. Concentration logic (`concentration.go`) has test
coverage. Spell events pattern is tested. No known logic bugs.

---

## Tools

### tools/spatial — B+

The load-bearing spatial math is well-covered. Cube coordinates, hex distance,
neighbor calculation, ring, spiral, and conversion round-trips are all tested in
`hex_grid_test.go` with suite pattern. Square grid covers distance (D&D 5e
Chebyshev), neighbors, adjacency, line of sight, and range queries.
`orchestrator_test.go` covers multi-room setup, connections, entity movement,
pathfinding, and layout types.

Two honest gaps:
1. The **in-room pathfinder (`SimplePathFinder`) only works on hex grids
   (CubeCoordinate).** There is no square-grid pathfinder for intra-room A*.
   The orchestrator's `FindPath` is room-to-room only. This means a monster
   navigating obstacles inside a square room has no toolkit path to follow.
2. `LayoutOrchestrator` and `TransitionSystem` interfaces are defined in
   `connection.go` with no implementations. They are documented as "future work"
   in CLAUDE.md but a reader of the interface file alone has no indication.
   `pathfinder_test.go` covers only 5 cases; no stress tests.

Grade would reach A- with a square-grid pathfinder and a "not implemented" comment
on the empty interfaces.

### tools/environments — B

Environment persistence (`ToData`/`LoadFromData`) is tested end-to-end including
cube coordinate validation and round-trip. `FindPathCube` is exercised with obstacle
and blocked-goal cases. `room_builder_test.go` and `wall_patterns_test.go` provide
useful coverage. Emergency fallback tested. Missing: no test for large environments
or environments with many passages; no test for `SelectablesTable` integration from
within environment generation. `graph_generator.go` is substantial but has no
direct unit tests — it is exercised only through environment creation.

### tools/selectables — B+

Weighted selection table with typed generics. `basic_table.go` is tested;
`events_test_simple.go` exists alongside `interfaces.go` and provides selectable
event coverage. `test_helpers.go` and `events_test_simple.go` (note: non-`_test.go`
suffix for the latter is intentional — it is a helper file for external tests, not
a test file itself). Clean design. Only gap: no test for degenerate weight
distribution (all zero, single item, overflow).

### tools/spawn — B

All four spawn phases (basic engine, advanced patterns, constraints, environment
integration) are tested. `basic_engine_test.go`, `constraints_test.go`, and
`environment_integration_test.go` all pass. The module depends on
`tools/spatial v0.2.1` and `tools/environments v0.1.2` — these are published
versions with no replace directives. Clean go.mod. Grade held at B because
`spawning_patterns.go` (formation, team, clustered) and `capacity_analysis.go`
have no standalone tests; they are exercised through the engine.

---

## Rulebooks

### rulebooks/dnd5e — B+

This is the most feature-complete and actively-worked module. All tests pass across
41 sub-packages. Integration tests for Barbarian, Fighter, Monk, and Rogue
encounters all pass. Character draft, equipment slots, combat, actions, conditions,
features, initiative, saves, spells, monsters, and monster traits all have
test coverage.

Known gaps that keep it from A:
1. **Several data-only sub-packages have no tests:** `abilities`, `ammunition`,
   `armor`, `backgrounds` (includes `grants.go` logic), `damage`, `effects`,
   `fightingstyles`, `items`, `languages`, `packs`, `proficiencies`, `race`,
   `races` (includes `grants.go` logic). The grant logic in `backgrounds/grants.go`
   and `races/grants.go` is non-trivial and untested.
2. **`dungeon/` lives inside the rulebook** but is architecturally slated to move
   out. Its test coverage is solid (336 lines of tests), but its location creates
   a coupling from rulebook → environments → spatial that bypasses the intended
   toolkit → rulebook layering.
3. **`character/choices` testdata provenance** is undocumented. The `testdata/api/`
   directory contains class and race JSON fixtures from an external API. No note
   on when this was fetched or how to refresh it.
4. **`combatabilities` dash, disengage, and dodge** are tested but `move.go` is
   tested minimally — no test for stopping reasons or multi-leg paths.

### rulebooks/dnd5e/combat — A-

The combat pipeline (AC, attack, damage, healing, movement, action economy,
turn manager) is thoroughly tested. `integration_test.go` and
`combatant_dirty_test.go` test cross-cutting concerns. `breakdown_test.go` ensures
the rich breakdown format required by the Boundary Rule is produced. The two-weapon
fighting test is its own file. Copilot review feedback has been addressed in recent
PRs. Only gap: no test for simultaneous multi-combatant AC resolution under
conditions.

---

## Items

### items — C

The base `items` module has **no test files** (only `validation/`).
`validation/basic_validator_test.go` now compiles (issue #612 resolved — mock
types updated to return `core.EntityType` instead of `string`). Replace
directive removed (issue #613 resolved 2026-05-04, pinned to `core v0.10.0`).
Held back from B by the absence of any tests at the base-module level.

---

## Grade legend

- **A** — strong design, good tests, no known gaps, clean go.mod
- **B** — works reliably; some known gaps, minor polish or hygiene issues
- **C** — meaningful gap: missing tests for non-trivial logic, or known regression
- **D** — tests broken or absent for load-bearing code; blocked from CI passing

## Grade distribution (2026-05-02)

| Grade | Modules |
|---|---|
| A / A- | core, rpgerr, dice, rulebooks/dnd5e/combat |
| B+ | game, events, mechanics/resources, tools/spatial, tools/selectables, rulebooks/dnd5e |
| B | mechanics/effects, mechanics/conditions, tools/environments, tools/spawn |
| B- | mechanics/proficiency, mechanics/spells |
| C | mechanics/features, items |

## How to use this doc

Grades are a starting point from 2026-05-02 read-through. When a grade changes,
record the reason. Don't just move the letter.

Suggested signals to watch:
- `go test ./...` in each module — catches mock-vs-interface drift like #612
- `go mod tidy` diff — catches the replace directive / go.sum drift
- New sub-packages with no test files — check grants.go in backgrounds/races
- Pathfinder coverage — add square-grid intra-room test before multi-room dungeon
  work begins in earnest
