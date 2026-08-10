---
name: tools/spatial module
description: Hex/Square/Gridless rooms, multi-room orchestration, spatial queries, pathfinding — second-largest rpg-api dependency
updated: 2026-08-10
confidence: high — verified by reading pathfinder.go, orchestrator.go, connection.go, hex_grid.go, square_grid.go, and rpg-api's hot-path imports per audit 049
---

# tools/spatial module

**Path:** `tools/spatial/`
**Module:** `github.com/KirkDiggler/rpg-toolkit/tools/spatial`
**Grade:** B+

The spatial infrastructure: where entities are, how far apart, how to move
between rooms. Does not implement game rules (movement costs, terrain effects,
attack of opportunity) — those belong in `rulebooks/dnd5e`.

## What rpg-api consumes

Per audit Section 1, `tools/spatial` is rpg-api's **second-heaviest toolkit
dependency** (18 files import it). The hot path:

| Symbol | Where rpg-api uses it | Histogram |
|---|---|---|
| `spatial.CubeCoordinate` | `internal/entities/merged_grid.go` | 128 |
| `spatial.RoomData` | `internal/entities/encounter_events.go` | 82 |
| `spatial.GridTypeHex` | `internal/handlers/dnd5e/v1alpha1/encounter/handler.go` | 52 |
| `spatial.EntityCubePlacement` | `internal/entities/merged_grid.go` | 45 |
| `spatial.Position` | `internal/entities/merged_grid.go` | 28 |
| `spatial.HexOrientationPointyTop` | encounter handler | 19 |
| `spatial.EntityPlacement` | merged_grid | 11 |

`RoomData`, `EntityCubePlacement`, and the grid-type/hex-orientation
constants are the dominant surface — rpg-api stores `spatial.RoomData`
directly (the toolkit's data type is canonical) and reasons in cube
coordinates throughout.

The orchestrator/pathfinder/connection types described below are toolkit
infrastructure. rpg-api currently does not import the multi-room
orchestrator — its dungeon graph lives in `tools/environments` (see
`tools-environments.md`).

## Files

| File | Purpose |
|---|---|
| `interfaces.go` | `Room`, `Placeable`, query, boundary, and event-bus interfaces |
| `room.go` | `BasicRoom` — entity placement, movement, queries |
| `hex_grid.go` | `HexGrid` (offset) and `AxialHexGrid` (Q/R) — hex distance, neighbors, and lines |
| `square_grid.go` | `SquareGrid` — Chebyshev distance, 8-neighbor grid |
| `gridless.go` | `GridlessRoom` — Euclidean, continuous positioning |
| `position.go` | `Position`, `CubeCoordinate`, `SquareCoord` types |
| `pathfinder.go` | `PathFinder` interface + `SimplePathFinder` (hex A*) |
| `orchestrator.go` | `Connection`, `RoomOrchestrator`, and layout-type contracts |
| `connection.go` | `Connection`, `BasicConnection` |
| `connection_helpers.go` | `CreateDoorConnection`, `CreateStairsConnection`, etc. |
| `basic_orchestrator.go` | `BasicRoomOrchestrator` room/connection/index implementation |
| `managed_membership.go` | Additive `ManagedRoomMutator` verbs and returned spatial deltas |
| `query_handler.go` | `SpatialQueryHandler` — multi-room entity queries |
| `query_utils.go` | Direct query utilities plus generic include/exclude filter helpers |
| `events.go` | Direct query data and `SimpleEntityFilter` |
| `topics.go` | Typed spatial event definitions and topics |
| `data.go` | `RoomData`, `EntityCubePlacement`, `EntityPlacement` — the serializable surface rpg-api stores |
| `ids.go` | `RoomID`, `ConnectionID`, and `OrchestratorID` constructors |

Managed entity membership uses canonical `core.EntityID`; spatial defines no parallel entity-ID type.

## Grid systems

The module exposes three grid shapes through four implementations:

| Implementation | `Position` contract | Bounds | Distance | Neighbors |
|---|---|---|---|---|
| `HexGrid` | offset column/row | non-negative Width/Height | cube hex distance after orientation-aware conversion | 6 |
| `AxialHexGrid` | axial Q/R | origin-centered SpanWidth/SpanHeight | `(abs(dQ) + abs(dR) + abs(dS)) / 2` | 6 |
| `SquareGrid` | x/y | non-negative Width/Height | Chebyshev: `max(abs(dx), abs(dy))` | 8 |
| `GridlessRoom` | continuous x/y | non-negative Width/Height | Euclidean | conceptual samples |

`HexGrid` and `AxialHexGrid` are not interchangeable aliases. `HexGrid`
honors `HexOrientationPointyTop` or `HexOrientationFlatTop`; `AxialHexGrid`
has no orientation configuration because its Q/R axes are intrinsic. Focused
tests pin both their distance and bounds differences.

## RoomData and EntityCubePlacement — the persistence shape

`spatial.RoomData` is the serializable form of a room. rpg-api stores instances
of this struct directly; there is no rpg-api-internal equivalent. That's
deliberate — the toolkit owns the spatial vocabulary, and the API's job is to
persist and pass through, not translate.

`spatial.EntityCubePlacement` carries an entity's hex position (cube
coordinates) and is used in `internal/entities/merged_grid.go` to track which
entities live where in the encounter grid.

## Multi-room orchestration

`BasicRoomOrchestrator` tracks multiple rooms and their connections.
`FindPath` is room-to-room (which sequence of rooms to traverse), not
intra-room.

Entity membership in managed rooms has one supported mutation seam:
`ManagedRoomMutator` (`PlaceEntity`, `MoveEntity`, `RemoveEntity`, and
`TransitionEntity`). Each verb returns a typed delta as a value and updates the
room plus entity-to-room index synchronously. Rooms used alone remain directly
mutable. After a room is added, direct mutation through a retained `Room` alias
(or sharing one room across orchestrators) is unsupported because Go interfaces
cannot enforce ownership and such bypasses can stale indexes.

The event bus is observer-only. `BasicRoomOrchestrator` does not subscribe to
`EntityPlacedTopic`, `EntityMovedTopic`, or `EntityRemovedTopic`; membership
correctness therefore does not depend on a bus, topic wiring order, or rooms and
orchestrator sharing a bus. Synchronous observers may re-enter read getters, but
hosts serialize managed mutations and re-entrant managed mutation from an
observer is outside the contract. `EntityRoomTransitionTopic` remains the active
observer notification for a successful managed `TransitionEntity` departure.
The unused began/ended transition lifecycle topics and progress-tracking shelf
were removed; there is no replacement notification channel.

Connections remain abstract and do not choose destination positions.
`TransitionEntity` consequently removes and unindexes the entity, then returns
the removed `core.Entity`, departure delta, and a transition with
`PlacementRequired=true`. The composition completes physical membership with a
later managed `PlaceEntity`. Until then, `GetEntityRoom` and
`CanMoveEntityBetweenRooms` both answer false. The legacy
`MoveEntityBetweenRooms` signature remains but discards this output and follows
the same corrected departure-only semantics.

Connection types (helper constructors in `connection_helpers.go`):
- `CreateDoorConnection` — standard bidirectional door
- `CreateStairsConnection` — vertical; one-way by default
- `CreatePassageConnection` — open hallway
- `CreatePortalConnection` — magical/instant
- `CreateBridgeConnection` — crossable gap
- `CreateTunnelConnection` — underground

## Known gaps

### PathFinder is hex-only (issue #614)

```go
type PathFinder interface {
    FindPath(start, goal CubeCoordinate, blocked map[CubeCoordinate]bool) []CubeCoordinate
}
```

`CubeCoordinate` is the hex type. There is no `SquarePathFinder` with
`SquareCoord` arguments. `SimplePathFinder.FindPath` uses `GetNeighbors()` on
`CubeCoordinate` — it has no knowledge of square grid topology.

A monster navigating obstacles inside a square room has no toolkit path. The
`BasicRoomOrchestrator.FindPath` returns a room sequence, not an intra-room
path. Callers must implement their own A* for square-grid intra-room
navigation. This is undocumented as a gap in the source. Fix: add
`SquarePathFinder` implementing `FindPath(start, goal SquareCoord, blocked
map[SquareCoord]bool) []SquareCoord`.

### Test coverage

`pathfinder_test.go` covers 8 cases, including direct and obstructed paths,
blocked endpoints, bounded traversal-predicate detours, and a sealed goal. No
tests cover large grids, cycles, or priority queue tie-breaking. For the current use case (small dungeon rooms)
this is acceptable, but it is worth noting before scaling to large
environments.

## go.mod status

Clean. Uses published versions for all dependencies:
- `core v0.11.0`
- `events v0.6.2`
- `game v0.1.0`
- `google/uuid v1.6.0`

No replace directives.

## Verification

```sh
# rpg-api's import surface
grep -rln '"github.com/KirkDiggler/rpg-toolkit/tools/spatial"' /home/kirk/personal/rpg-api/internal/ /home/kirk/personal/rpg-api/cmd/ --include="*.go" | wc -l   # expect 18

# Symbol histogram for the hot path
grep -roE 'spatial\.(RoomData|EntityCubePlacement|GridTypeHex|HexOrientationPointyTop|CubeCoordinate)' /home/kirk/personal/rpg-api/internal/ /home/kirk/personal/rpg-api/cmd/ --include="*.go" | sort | uniq -c | sort -rn | head

# Toolkit module surface
grep -nE "^func [A-Z]|^type [A-Z]" /home/kirk/personal/rpg-toolkit/tools/spatial/data.go
```
