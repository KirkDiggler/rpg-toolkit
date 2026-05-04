---
name: tools/environments module
description: Multi-room environment graph, persistence, generation — rpg-api consumes a narrow slice (ConnectionEdge, RoomShape, GetDefaultShapes, ConnectionPoint)
updated: 2026-05-04
confidence: high — verified by reading environment_persistence.go, environment_data.go, graph_generator.go, and rpg-api dungeon-entity callsites per audit 049
---

# tools/environments module

**Path:** `tools/environments/`
**Module:** `github.com/KirkDiggler/rpg-toolkit/tools/environments`
**Grade:** B

Builds on `tools/spatial` to provide a multi-room environment graph: how rooms
connect (passages, doors), how to persist the entire dungeon layout, and how
to find paths across connected rooms.

## What rpg-api consumes

Per audit Section 1, rpg-api imports `tools/environments` from 6 files but
uses **only** a narrow slice of the surface:

| Symbol | Where rpg-api uses it | Histogram |
|---|---|---|
| `environments.ConnectionEdge` | `internal/entities/dungeon.go` | 10 |
| `environments.RoomShape` | dungeon entity | 5 |
| `environments.GetDefaultShapes` | dungeon construction | 1 |
| `environments.ConnectionPoint` | dungeon entity | 1 |

rpg-api uses environments for **dungeon-graph data only** — storing which
rooms connect via which edges, and what shape each room is. The bulk of this
module (graph generators, wall patterns, environment persistence,
pathfinding) is **toolkit-internal infrastructure** that rpg-api does not
import directly. Most of what's documented below is "what the toolkit can do
for you," not "what rpg-api currently uses."

## Files (key ones)

| File | Purpose |
|---|---|
| `environment.go` | `BasicEnvironment` — the environment aggregate |
| `environment_data.go` | `EnvironmentData`, `RoomData`, `PassageData`, `OriginData` |
| `environment_persistence.go` | `ToData()`, `LoadFromData()`, absolute-position entity tracking |
| `graph_generator.go` | Procedural room graph generation algorithms (toolkit-internal) |
| `room_builder.go` | Fluent API for constructing rooms with walls, passages |
| `wall_patterns.go` | Pre-defined wall configurations (perimeter, cross, T-junction) (toolkit-internal) |
| `pathfinding.go` | `FindPathCube` — hex A* across connected rooms (toolkit-internal) |

## Persistence pattern (toolkit-internal)

`BasicEnvironment` is stateful — it holds rooms, connections, and entity
positions. It follows the `ToData`/`LoadFromData` convention:

```go
// Serialize: all room positions converted to dungeon-absolute coordinates
data := env.ToData()

// Reconstitute: absolute positions restored, rooms reconnected
env, err := environments.LoadFromData(ctx, LoadFromDataInput{Data: data, EventBus: bus})
```

Entity positions are stored in absolute (dungeon-wide) coordinates in
`EnvironmentData`, not room-local coordinates. The persistence layer converts
between absolute and room-local on save/load.

rpg-api does not currently exercise this persistence path. Its dungeon entity
stores `[]*environments.ConnectionEdge` directly and reconstructs the graph
itself.

## Pathfinding (toolkit-internal)

`FindPathCube(start, goal CubeCoordinate) ([]CubeCoordinate, error)` finds a
path in cube (hex) coordinates across connected rooms. It uses the hex
pathfinder from `tools/spatial`. This is the authoritative cross-room
pathfinder for hex-grid dungeons.

**Square-grid cross-room pathfinding:** Not provided. Same gap as in
`tools/spatial` — there is no square-grid pathfinder at any layer.

## Graph generation (toolkit-internal)

`graph_generator.go` is substantial (~400 lines). It generates connected room
graphs with configurable parameters (room count, connection density,
branching factor). **It has no direct unit tests.** It is tested indirectly
through the `BasicEnvironment` creation tests in
`environment_persistence_test.go`.

## go.mod status

Clean. Published versions, no replace directives.

## Known gaps

- `graph_generator.go` is load-bearing but untested in isolation.
- No test for `SelectablesTable` integration within environment generation (mentioned as "missing" in `quality.md`).
- Large environments (100+ rooms) are not load-tested. Performance is extrapolated from small (4-8 room) tests.

## Verification

```sh
# rpg-api's narrow import surface
grep -rln '"github.com/KirkDiggler/rpg-toolkit/tools/environments"' /home/kirk/personal/rpg-api/internal/ /home/kirk/personal/rpg-api/cmd/ --include="*.go" | wc -l   # expect 6

# Specific symbols rpg-api consumes
grep -rn 'environments\.\(ConnectionEdge\|RoomShape\|ConnectionPoint\|GetDefaultShapes\)' /home/kirk/personal/rpg-api/internal/ --include="*.go" | head
```
