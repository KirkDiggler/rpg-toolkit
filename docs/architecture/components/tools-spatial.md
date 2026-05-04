---
name: tools/spatial module
description: Hex/Square/Gridless room management, multi-room orchestration, spatial queries, pathfinding
updated: 2026-05-02
confidence: high — verified by reading pathfinder.go, orchestrator.go, connection.go, hex_grid.go, square_grid.go, and test files
---

# tools/spatial module

**Path:** `tools/spatial/`
**Module:** `github.com/KirkDiggler/rpg-toolkit/tools/spatial`
**Grade:** B+

The spatial infrastructure: where entities are, how far apart, how to move between rooms. Does not implement game rules (movement costs, terrain effects, attack of opportunity) — those belong in `rulebooks/dnd5e`.

## Files

| File | Purpose |
|---|---|
| `interfaces.go` | `Room`, `Orchestrator`, `Placeable` interfaces |
| `room.go` | `BasicRoom` — entity placement, movement, queries |
| `hex_grid.go` | `HexGrid` — cube coordinates, distance, neighbors, ring, spiral |
| `square_grid.go` | `SquareGrid` — Chebyshev distance, 8-neighbor grid |
| `gridless.go` | `GridlessRoom` — Euclidean, continuous positioning |
| `position.go` | `Position`, `CubeCoordinate`, `SquareCoord` types |
| `pathfinder.go` | `PathFinder` interface + `SimplePathFinder` (hex A*) |
| `orchestrator.go` | `Orchestrator` interface + `BasicRoomOrchestrator`, `LayoutOrchestrator` (unimplemented), `TransitionSystem` (unimplemented) |
| `connection.go` | `Connection`, `BasicConnection` |
| `connection_helpers.go` | `CreateDoorConnection`, `CreateStairsConnection`, etc. |
| `basic_orchestrator.go` | `BasicRoomOrchestrator` implementation |
| `query_handler.go` | `SpatialQueryHandler` — multi-room entity queries |
| `query_utils.go` | Filter helpers (`CreateCharacterFilter`, `CreateMonsterFilter`, etc.) |
| `events.go` | Event types: `EntityPlacedEvent`, `EntityMovedEvent`, `RoomAddedEvent`, etc. |
| `topics.go` | Typed topic definitions |
| `data.go` | `RoomData` — serializable room state |
| `ids.go` | Typed ID constants |

## Grid systems

All three grid types are fully implemented:

| Grid | Coordinate type | Distance | Neighbors |
|---|---|---|---|
| Hex | `CubeCoordinate` (q, r, s) | `(abs(q) + abs(r) + abs(s)) / 2` | 6 |
| Square | `SquareCoord` (x, y) | Chebyshev: `max(abs(dx), abs(dy))` | 8 |
| Gridless | `Position` (float64 x, y) | Euclidean | N/A |

Each grid implements its own distance calculation. `Position` is a data type; the grid decides the math.

## Multi-room orchestration

`BasicRoomOrchestrator` tracks multiple rooms and their connections. `FindPath` is room-to-room (which sequence of rooms to traverse), not intra-room.

Connection types (helper constructors in `connection_helpers.go`):
- `CreateDoorConnection` — standard bidirectional door
- `CreateStairsConnection` — vertical; one-way by default
- `CreatePassageConnection` — open hallway
- `CreatePortalConnection` — magical/instant
- `CreateBridgeConnection` — crossable gap
- `CreateTunnelConnection` — underground

## Known gaps

### PathFinder is hex-only (issue #614)
`pathfinder.go:9`:
```go
type PathFinder interface {
    FindPath(start, goal CubeCoordinate, blocked map[CubeCoordinate]bool) []CubeCoordinate
}
```

`CubeCoordinate` is the hex type. There is no `SquarePathFinder` with `SquareCoord` arguments. `SimplePathFinder.FindPath` uses `GetNeighbors()` on `CubeCoordinate` — it has no knowledge of square grid topology.

A monster navigating obstacles inside a square room has no toolkit path. The `BasicRoomOrchestrator.FindPath` returns a room sequence, not an intra-room path. Callers must implement their own A* for square-grid intra-room navigation. This is undocumented as a gap in the source. Fix: add `SquarePathFinder` implementing `FindPath(start, goal SquareCoord, blocked map[SquareCoord]bool) []SquareCoord`.

### Unimplemented interfaces with no marker (issue #614 adjacent)
`orchestrator.go:109` and `orchestrator.go:135` define `LayoutOrchestrator` and `TransitionSystem`:
- `LayoutOrchestrator` — auto-position rooms, calculate layout metrics
- `TransitionSystem` — track in-progress entity transitions between rooms

Both are defined but have no implementation in this package. `tools/spatial/CLAUDE.md` documents them as "future work," but a reader of `orchestrator.go` alone has no indication. There is no `// Not implemented` comment, no `var _ LayoutOrchestrator = (*notImplemented)(nil)` guard, nothing. Risk: a new contributor implements them incorrectly assuming an interface contract that is actually advisory.

### Test coverage
`pathfinder_test.go` covers 5 cases: direct path, L-shaped wall, surrounded (no path), same position, blocked goal. No tests for large grids, cycles, or priority queue tie-breaking. For the current use case (small dungeon rooms) this is acceptable, but it is worth noting before scaling to large environments.

## go.mod status
Clean. Uses published versions for all dependencies:
- `core v0.9.6`
- `events v0.6.2`
- `game v0.1.0`
- `google/uuid v1.6.0`

No replace directives.
