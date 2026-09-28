# Spatial Module

## Purpose

2D spatial positioning and movement infrastructure **without game-specific
rules**: grid systems (square, hex, gridless), entity placement and movement,
multi-room orchestration, spatial queries and line of sight.

We implement: the mathematical foundation. Game implementations decide: how to
use spatial data, what entities can do, rule interpretations — movement costs
and terrain rules belong to the game layer.

Mechanics, persistence shapes, orchestration semantics, connection types,
worked patterns and pitfalls live in
`docs/architecture/components/tools-spatial.md`. The `README.md` beside this
file is the authoritative usage guide — keep it in sync with code changes.

## Laws

- **Spatial is infrastructure, not game rules.** A new capability here answers
  "where is it / how far apart"; what entities may do with that belongs to
  the rulebook.
- **Connections are abstract links between rooms, not physical objects**
  (ADR-0015). Connections carry no position; the game layer places door
  entities at the linked positions.
- **The event bus is optional and observer-only, and connecting is separate
  from creation.** A standalone room mutates directly and publishes nothing
  until `ConnectToEventBus`. Once a room is added to an orchestrator, mutate
  only through its `ManagedRoomMutator` verbs — retained-room mutation and
  sharing one room across orchestrators are unsupported alias bypasses that
  stale indexes.
- **Thread safety is built in** (RWMutex; orchestrator locks are released
  before room calls and event publication; triple-tracking indexes for
  entities, positions and occupancy). Do not add extra locks; hosts serialize
  managed mutations while concurrent reads stay safe.
- **Grid types are distinct public contracts.** `HexGrid` (offset, with
  pointy/flat orientation) and `AxialHexGrid` (Q/R, no orientation) are not
  interchangeable — never feed one the other's positions, and do not
  consolidate or rename them without an explicit migration.
- **The grid owns the math.** `Position` does not enforce distance; each grid
  implementation handles its own distance and neighbors.
- **Imports follow the events ordering standard** — testify (third-party)
  before local imports, enforced by goimports.
- **All public APIs carry comments** naming purpose, when events publish, and
  error conditions.

## Pointers

- Mechanics, files, gaps: `docs/architecture/components/tools-spatial.md`
- ADR-0015 (abstract connections): `docs/adr/`
- Test commands and the testify suite pattern: `docs/how-to/run-tests.md`
- Current health: `docs/status.md`